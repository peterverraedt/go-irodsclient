package connection

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/cyverse/go-irodsclient/irods/auth"
	"github.com/cyverse/go-irodsclient/irods/message"
	"github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/go-irodsclient/irods/util"
)

// IRODSConnection connects to iRODS
type IRODSConnection struct {
	Account         *types.IRODSAccount
	Timeout         time.Duration
	ApplicationName string

	// internal
	connected         bool
	socket            net.Conn
	serverVersion     *types.IRODSVersion
	generatedPassword string
}

// NewIRODSConnection create a IRODSConnection
func NewIRODSConnection(account *types.IRODSAccount, timeout time.Duration, applicationName string) *IRODSConnection {
	return &IRODSConnection{
		Account:         account,
		Timeout:         timeout,
		ApplicationName: applicationName,
	}
}

// GetVersion returns iRODS version
func (conn *IRODSConnection) GetVersion() *types.IRODSVersion {
	return conn.serverVersion
}

func (conn *IRODSConnection) requiresCSNegotiation() bool {
	return conn.Account.ClientServerNegotiation
}

// IsConnected returns if the connection is live
func (conn *IRODSConnection) IsConnected() bool {
	return conn.connected
}

// Connect connects to iRODS
func (conn *IRODSConnection) Connect() error {
	conn.connected = false

	server := fmt.Sprintf("%s:%d", conn.Account.Host, conn.Account.Port)
	socket, err := net.Dial("tcp", server)
	if err != nil {
		return fmt.Errorf("Could not connect to specified host and port (%s:%d) - %v", conn.Account.Host, conn.Account.Port, err)
	}

	conn.socket = socket
	var irodsVersion *types.IRODSVersion
	if conn.requiresCSNegotiation() {
		// client-server negotiation
		util.LogInfo("Connect with CS Negotiation")
		irodsVersion, err = conn.connectWithCSNegotiation()
	} else {
		// No client-server negotiation
		util.LogInfo("Connect without CS Negotiation")
		irodsVersion, err = conn.connectWithoutCSNegotiation()
	}

	if err != nil {
		_ = conn.disconnectNow()
		return err
	}

	conn.serverVersion = irodsVersion

	switch conn.Account.AuthenticationScheme {
	case types.AuthSchemeNative:
		err = conn.loginNative(conn.Account.Password)
	case types.AuthSchemeGSI:
		err = conn.loginGSI()
	case types.AuthSchemePAM:
		err = conn.loginPAM()
	default:
		return fmt.Errorf("Unknown Authentication Scheme - %s", conn.Account.AuthenticationScheme)
	}

	if err != nil {
		_ = conn.disconnectNow()
		return err
	}

	conn.connected = true

	return nil
}

func (conn *IRODSConnection) connectWithCSNegotiation() (*types.IRODSVersion, error) {
	// Get client negotiation policy
	clientPolicy := types.CSNegotiationRequireTCP
	if len(conn.Account.CSNegotiationPolicy) > 0 {
		clientPolicy = conn.Account.CSNegotiationPolicy
	}

	// Send a startup message
	util.LogInfo("Start up a new connection")
	startup := message.NewIRODSMessageStartupPack(conn.Account, conn.ApplicationName, true)
	startupMessage, err := startup.GetMessage()
	if err != nil {
		return nil, fmt.Errorf("Could not make a startup message - %v", err)
	}

	err = conn.SendMessage(startupMessage)
	if err != nil {
		return nil, fmt.Errorf("Could not send a startup message - %v", err)
	}

	// Server responds with negotiation response
	negotiationMessage, err := conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("Could not receive a negotiation message - %v", err)
	}

	if negotiationMessage.Body == nil {
		return nil, fmt.Errorf("Could not receive a negotiation message body")
	}

	if negotiationMessage.Body.Type == message.RODS_MESSAGE_VERSION_TYPE {
		// this happens when an error occur
		// Server responds with version
		version := message.IRODSMessageVersion{}
		err = version.FromMessage(negotiationMessage)
		if err != nil {
			return nil, fmt.Errorf("Could not receive a negotiation message - %v", err)
		}

		return version.GetVersion(), nil
	} else if negotiationMessage.Body.Type == message.RODS_MESSAGE_CS_NEG_TYPE {
		// Server responds with its own negotiation policy
		util.LogInfo("Start up CS Negotiation")
		negotiation := message.IRODSMessageCSNegotiation{}
		err = negotiation.FromMessage(negotiationMessage)
		if err != nil {
			return nil, fmt.Errorf("Could not receive a negotiation message - %v", err)
		}

		serverPolicy, err := types.GetCSNegotiationRequire(negotiation.Result)
		if err != nil {
			return nil, fmt.Errorf("Unable to parse server policy - %v", err)
		}

		// Perform the negotiation
		policyResult, status := types.PerformCSNegotiation(clientPolicy, serverPolicy)

		// If negotiation failed we're done
		if policyResult == types.CSNegotiationFailure {
			return nil, fmt.Errorf("Client-Server negotiation failed: %s, %s", string(clientPolicy), string(serverPolicy))
		}

		// Send negotiation result to server
		negotiationResult := message.NewIRODSMessageCSNegotiation(status, policyResult)
		negotiationResultMessage, err := negotiationResult.GetMessage()
		if err != nil {
			return nil, fmt.Errorf("Could not make a negotiation result message - %v", err)
		}

		err = conn.SendMessage(negotiationResultMessage)
		if err != nil {
			return nil, fmt.Errorf("Could not send a negotiation result message - %v", err)
		}

		// Server responds with version
		versionMessage, err := conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("Could not receive a version message - %v", err)
		}

		version := message.IRODSMessageVersion{}
		err = version.FromMessage(versionMessage)
		if err != nil {
			return nil, fmt.Errorf("Could not receive a version message - %v", err)
		}

		if policyResult == types.CSNegotiationUseSSL {
			err := conn.sslStartup()
			if err != nil {
				return nil, fmt.Errorf("Could not start up SSL - %v", err)
			}
		}

		return version.GetVersion(), nil
	}

	return nil, fmt.Errorf("Unknown response message - %s", negotiationMessage.Body.Type)
}

func (conn *IRODSConnection) connectWithoutCSNegotiation() (*types.IRODSVersion, error) {
	// No client-server negotiation
	// Send a startup message
	util.LogInfo("Start up a new connection")
	startup := message.NewIRODSMessageStartupPack(conn.Account, conn.ApplicationName, false)
	startupMessage, err := startup.GetMessage()
	if err != nil {
		return nil, fmt.Errorf("Could not make a startup message - %v", err)
	}

	err = conn.SendMessage(startupMessage)
	if err != nil {
		return nil, fmt.Errorf("Could not send a startup message - %v", err)
	}

	// Server responds with version
	versionMessage, err := conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("Could not receive a version message - %v", err)
	}

	version := message.IRODSMessageVersion{}
	err = version.FromMessage(versionMessage)
	if err != nil {
		return nil, fmt.Errorf("Could not receive a version message - %v", err)
	}

	return version.GetVersion(), nil
}

func (conn *IRODSConnection) sslStartup() error {
	util.LogInfo("Start up SSL")

	irodsSSLConfig := conn.Account.SSLConfiguration
	if irodsSSLConfig == nil {
		return fmt.Errorf("SSL Configuration is not set")
	}

	caCertPool := x509.NewCertPool()
	caCert, err := irodsSSLConfig.ReadCACert()
	if err == nil {
		caCertPool.AppendCertsFromPEM(caCert)
	}

	sslConf := &tls.Config{
		RootCAs:    caCertPool,
		ServerName: conn.Account.Host,
	}

	// Create a side connection using the existing socket
	sslSocket := tls.Client(conn.socket, sslConf)

	err = sslSocket.Handshake()
	if err != nil {
		return fmt.Errorf("SSL Handshake error - %v", err)
	}

	// from now on use ssl socket
	conn.socket = sslSocket

	// Generate a key (shared secret)
	encryptionKey := make([]byte, irodsSSLConfig.EncryptionKeySize)
	_, err = rand.Read(encryptionKey)
	if err != nil {
		return fmt.Errorf("Could not generate a shared secret - %v", err)
	}

	// Send a ssl setting
	sslSetting := message.NewIRODSMessageSSLSettings(irodsSSLConfig.EncryptionAlgorithm, irodsSSLConfig.EncryptionKeySize, irodsSSLConfig.SaltSize, irodsSSLConfig.HashRounds)
	sslSettingMessage, err := sslSetting.GetMessage()
	if err != nil {
		return fmt.Errorf("Could not make a ssl setting message - %v", err)
	}

	err = conn.SendMessage(sslSettingMessage)
	if err != nil {
		return fmt.Errorf("Could not send a ssl setting message - %v", err)
	}

	// Send a shared secret
	sslSharedSecret := message.NewIRODSMessageSSLSharedSecret(encryptionKey)
	sslSharedSecretMessage, err := sslSharedSecret.GetMessage()
	if err != nil {
		return fmt.Errorf("Could not make a ssl shared secret message - %v", err)
	}

	err = conn.SendMessage(sslSharedSecretMessage)
	if err != nil {
		return fmt.Errorf("Could not send a ssl shared secret message - %v", err)
	}

	return nil
}

func (conn *IRODSConnection) loginNative(password string) error {
	util.LogInfo("Logging in using native authentication method")

	// authenticate
	authRequest := message.NewIRODSMessageAuthRequest()
	authRequestMessage, err := authRequest.GetMessage()
	if err != nil {
		return fmt.Errorf("Could not make a login request message - %v", err)
	}

	err = conn.SendMessage(authRequestMessage)
	if err != nil {
		return fmt.Errorf("Could not send a login request message - %v", err)
	}

	// challenge
	authChallengeMessage, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("Could not receive an authentication challenge message - %v", err)
	}

	authChallenge := message.IRODSMessageAuthChallenge{}
	err = authChallenge.FromMessage(authChallengeMessage)
	if err != nil {
		return fmt.Errorf("Could not receive an authentication challenge message body")
	}

	encodedPassword, err := auth.GenerateAuthResponse(authChallenge.Challenge, password)
	if err != nil {
		return fmt.Errorf("Could not generate an authentication response - %v", err)
	}

	authResponse := message.NewIRODSMessageAuthResponse(encodedPassword, conn.Account.ProxyUser)
	authResponseMessage, err := authResponse.GetMessage()
	if err != nil {
		return fmt.Errorf("Could not make a login response message - %v", err)
	}

	err = conn.SendMessage(authResponseMessage)
	if err != nil {
		return fmt.Errorf("Could not send a login response message - %v", err)
	}

	authResultMessage, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("Could not receive a login result message - %v", err)
	}

	authResult := message.IRODSMessageAuthResult{}
	err = authResult.FromMessage(authResultMessage)
	if err != nil {
		return fmt.Errorf("Could not receive a login result message body - %v", err)
	}

	err = authResult.CheckError()
	return err
}

func (conn *IRODSConnection) loginGSI() error {
	return nil
}

func (conn *IRODSConnection) loginPAM() error {
	util.LogInfo("Logging in using pam authentication method")

	// Check whether ssl has already started, if not, start ssl.
	if _, ok := conn.socket.(*tls.Conn); !ok {
		return fmt.Errorf("connection should be using SSL")
	}

	ttl := conn.Account.PamTTL
	if ttl <= 0 {
		ttl = 1
	}

	// authenticate
	pamAuthRequest := message.NewIRODSMessagePamAuthRequest(conn.Account.ClientUser, conn.Account.Password, ttl)
	pamAuthRequestMessage, err := pamAuthRequest.GetMessage()
	if err != nil {
		return fmt.Errorf("Could not make a pam login request message - %v", err)
	}

	err = conn.SendMessage(pamAuthRequestMessage)
	if err != nil {
		return fmt.Errorf("Could not send a pam login request message - %v", err)
	}

	// response
	pamAuthResponseMessage, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("Could not receive an authentication challenge message - %v", err)
	}

	pamAuthResponse := message.IRODSMessagePamAuthResponse{}
	err = pamAuthResponse.FromMessage(pamAuthResponseMessage)
	if err != nil {
		return fmt.Errorf("Could not receive an authentication challenge message body")
	}

	// save irods generated password for possible future use
	conn.generatedPassword = pamAuthResponse.GeneratedPassword

	// retry native auth with generated password
	return conn.loginNative(conn.generatedPassword)
}

// Disconnect disconnects
func (conn *IRODSConnection) disconnectNow() error {
	conn.connected = false
	err := conn.socket.Close()
	conn.socket = nil
	return err
}

// Disconnect disconnects
func (conn *IRODSConnection) Disconnect() error {
	util.LogInfo("Disconnecting")
	disconnect := message.NewIRODSMessageDisconnect()
	disconnectMessage, err := disconnect.GetMessage()
	if err != nil {
		return fmt.Errorf("Could not make a disconnect request message - %v", err)
	}

	err = conn.SendMessage(disconnectMessage)
	if err != nil {
		return fmt.Errorf("Could not send a disconnect request message - %v", err)
	}

	return conn.disconnectNow()
}

// Send sends data
func (conn *IRODSConnection) Send(buffer []byte, size int) error {
	// use sslSocket
	if conn.Timeout > 0 {
		conn.socket.SetWriteDeadline(time.Now().Add(conn.Timeout))
	}

	err := util.WriteBytes(conn.socket, buffer, size)
	if err != nil {
		util.LogError("Unable to send data. " +
			"Connection to remote host may have closed. " +
			"Releasing connection from pool.")
		conn.release(true)
		return fmt.Errorf("Unable to send data - %v", err)
	}
	return nil
}

// Recv receives a message
func (conn *IRODSConnection) Recv(buffer []byte, size int) (int, error) {
	if conn.Timeout > 0 {
		conn.socket.SetReadDeadline(time.Now().Add(conn.Timeout))
	}

	readLen, err := util.ReadBytes(conn.socket, buffer, size)
	if err != nil {
		util.LogError("Unable to receive data. " +
			"Connection to remote host may have closed. " +
			"Releasing connection from pool.")
		conn.release(true)
		return readLen, fmt.Errorf("Unable to receive data - %v", err)
	}
	return readLen, nil
}

// SendMessage makes the message into bytes
func (conn *IRODSConnection) SendMessage(msg *message.IRODSMessage) error {
	messageBuffer := new(bytes.Buffer)

	if msg.Header == nil && msg.Body == nil {
		return fmt.Errorf("Header and Body cannot be nil")
	}

	var headerBytes []byte
	var err error

	messageLen := 0
	errorLen := 0
	bsLen := 0

	if msg.Body != nil {
		if msg.Body.Message != nil {
			messageLen = len(msg.Body.Message)
		}

		if msg.Body.Error != nil {
			errorLen = len(msg.Body.Error)
		}

		if msg.Body.Bs != nil {
			bsLen = len(msg.Body.Bs)
		}

		if msg.Header == nil {
			h := message.MakeIRODSMessageHeader(msg.Body.Type, uint32(messageLen), uint32(errorLen), uint32(bsLen), msg.Body.IntInfo)
			headerBytes, err = h.GetBytes()
			if err != nil {
				return err
			}
		}
	}

	if msg.Header != nil {
		headerBytes, err = msg.Header.GetBytes()
		if err != nil {
			return err
		}
	}

	// pack length - Big Endian
	headerLenBuffer := make([]byte, 4)
	binary.BigEndian.PutUint32(headerLenBuffer, uint32(len(headerBytes)))

	// header
	messageBuffer.Write(headerLenBuffer)
	messageBuffer.Write(headerBytes)

	if msg.Body != nil {
		bodyBytes, err := msg.Body.GetBytes()
		if err != nil {
			return err
		}

		// body
		messageBuffer.Write(bodyBytes)
	}

	// send
	bytes := messageBuffer.Bytes()
	conn.Send(bytes, len(bytes))
	return nil
}

// readMessageHeader reads data from the given connection and returns iRODSMessageHeader
func (conn *IRODSConnection) readMessageHeader() (*message.IRODSMessageHeader, error) {
	// read header size
	headerLenBuffer := make([]byte, 4)
	readLen, err := conn.Recv(headerLenBuffer, 4)
	if readLen != 4 {
		return nil, fmt.Errorf("Could not read header size")
	}
	if err != nil {
		return nil, fmt.Errorf("Could not read header size - %v", err)
	}

	headerSize := binary.BigEndian.Uint32(headerLenBuffer)
	if headerSize <= 0 {
		return nil, fmt.Errorf("Invalid header size returned - len = %d", headerSize)
	}

	// read header
	headerBuffer := make([]byte, headerSize)
	readLen, err = conn.Recv(headerBuffer, int(headerSize))
	if err != nil {
		return nil, fmt.Errorf("Could not read header - %v", err)
	}
	if readLen != int(headerSize) {
		return nil, fmt.Errorf("Could not read header fully - %d requested but %d read", headerSize, readLen)
	}

	header := message.IRODSMessageHeader{}
	err = header.FromBytes(headerBuffer)
	if err != nil {
		return nil, err
	}

	return &header, nil
}

// ReadMessage reads data from the given socket and returns IRODSMessage
func (conn *IRODSConnection) ReadMessage() (*message.IRODSMessage, error) {
	header, err := conn.readMessageHeader()
	if err != nil {
		return nil, err
	}

	// read body
	bodyLen := header.MessageLen + header.ErrorLen + header.BsLen
	bodyBuffer := make([]byte, bodyLen)

	readLen, err := conn.Recv(bodyBuffer, int(bodyLen))
	if err != nil {
		return nil, fmt.Errorf("Could not read body - %v", err)
	}
	if readLen != int(bodyLen) {
		return nil, fmt.Errorf("Could not read body fully - %d requested but %d read", bodyLen, readLen)
	}

	body := message.IRODSMessageBody{}
	err = body.FromBytes(header, bodyBuffer)
	if err != nil {
		return nil, err
	}

	body.Type = header.Type
	body.IntInfo = header.IntInfo

	return &message.IRODSMessage{
		Header: header,
		Body:   &body,
	}, nil
}

func (conn *IRODSConnection) release(val bool) {
}
