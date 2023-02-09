package testcases

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
	"os"
	"path"
	"testing"
	"time"

	"github.com/cyverse/go-irodsclient/irods/connection"
	"github.com/cyverse/go-irodsclient/irods/fs"
	"github.com/cyverse/go-irodsclient/irods/session"
	"github.com/cyverse/go-irodsclient/irods/types"
	"github.com/rs/xid"
	"github.com/stretchr/testify/assert"

	log "github.com/sirupsen/logrus"
)

var (
	fsAPITestID = xid.New().String()
)

func TestFSAPI(t *testing.T) {
	setup()
	defer shutdown()

	log.SetLevel(log.DebugLevel)

	makeHomeDir(t, fsAPITestID)

	t.Run("test PrepareSamples", testPrepareSamplesForFSAPI)
	t.Run("test GetIRODSCollection", testGetIRODSCollection)
	t.Run("test ListIRODSCollections", testListIRODSCollections)
	t.Run("test ListIRODSCollectionMeta", testListIRODSCollectionMeta)
	t.Run("test ListIRODSCollectionAccess", testListIRODSCollectionAccess)
	t.Run("test ListIRODSDataObjects", testListIRODSDataObjects)
	t.Run("test ListIRODSDataObjectsMasterReplica", testListIRODSDataObjectsMasterReplica)
	t.Run("test GetIRODSDataObject", testGetIRODSDataObject)
	t.Run("test GetIRODSDataObjectMasterReplica", testGetIRODSDataObjectMasterReplica)
	t.Run("test ListIRODSDataObjectMeta", testListIRODSDataObjectMeta)
	t.Run("test ListIRODSDataObjectAccess", testListIRODSDataObjectAccess)
	t.Run("test CreateDeleteIRODSCollection", testCreateDeleteIRODSCollection)
	t.Run("test CreateMoveDeleteIRODSCollection", testCreateMoveDeleteIRODSCollection)
	t.Run("test CreateDeleteIRODSDataObject", testCreateDeleteIRODSDataObject)
	t.Run("test ReadWriteIRODSDataObject", testReadWriteIRODSDataObject)
	t.Run("test ReadWriteIRODSDataObjectWithSingleConnection", testReadWriteIRODSDataObjectWithSingleConnection)
	t.Run("test MixedReadWriteIRODSDataObjectWithSingleConnection", testMixedReadWriteIRODSDataObjectWithSingleConnection)
	t.Run("test TruncateIRODSDataObject", testTruncateIRODSDataObject)
	t.Run("test ListIRODSGroupUsers", testListIRODSGroupUsers)
	t.Run("test SearchDataObjectsByMeta", testSearchDataObjectsByMeta)
	t.Run("test SearchDataObjectsByMetaWildcard", testSearchDataObjectsByMetaWildcard)
	t.Run("test ParallelUploadAndDownloadDataObject", testParallelUploadAndDownloadDataObject)
}

func testPrepareSamplesForFSAPI(t *testing.T) {
	prepareSamples(t, fsAPITestID)
}

func testGetIRODSCollection(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	homedir := getHomeDir(fsAPITestID)

	collection, err := fs.GetCollection(conn, homedir)
	assert.NoError(t, err)

	assert.Equal(t, homedir, collection.Path)
	assert.NotEmpty(t, collection.ID)
}

func testListIRODSCollections(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	homedir := getHomeDir(fsAPITestID)

	collections, err := fs.ListSubCollections(conn, homedir)
	assert.NoError(t, err)

	collectionPaths := []string{}

	for _, collection := range collections {
		collectionPaths = append(collectionPaths, collection.Path)
		assert.NotEmpty(t, collection.ID)
	}

	assert.Equal(t, len(collections), len(GetTestDirs()))
	assert.ElementsMatch(t, collectionPaths, GetTestDirs())
}

func testListIRODSCollectionMeta(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	homedir := getHomeDir(fsAPITestID)

	metas, err := fs.ListCollectionMeta(conn, homedir)
	assert.NoError(t, err)

	for _, meta := range metas {
		assert.NotEmpty(t, meta.AVUID)
	}
}

func testListIRODSCollectionAccess(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	homedir := getHomeDir(fsAPITestID)

	accesses, err := fs.ListCollectionAccesses(conn, homedir)
	assert.NoError(t, err)

	for _, access := range accesses {
		assert.NotEmpty(t, access.Path)
		assert.Equal(t, homedir, access.Path)
	}
}

func testListIRODSDataObjects(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	homedir := getHomeDir(fsAPITestID)

	collection, err := fs.GetCollection(conn, homedir)
	assert.NoError(t, err)
	assert.NotEmpty(t, collection.ID)

	dataobjects, err := fs.ListDataObjects(conn, collection)
	assert.NoError(t, err)

	dataobjectPaths := []string{}

	for _, dataobject := range dataobjects {
		dataobjectPaths = append(dataobjectPaths, dataobject.Path)
		assert.NotEmpty(t, dataobject.ID)

		assert.Equal(t, string(types.GENERIC_DT), dataobject.DataType)

		for _, replica := range dataobject.Replicas {
			assert.NotEmpty(t, replica.Path)
		}
	}

	assert.Equal(t, len(dataobjects), len(GetTestFiles()))
	assert.ElementsMatch(t, dataobjectPaths, GetTestFiles())
}

func testListIRODSDataObjectsMasterReplica(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	homedir := getHomeDir(fsAPITestID)

	collection, err := fs.GetCollection(conn, homedir)
	assert.NoError(t, err)
	assert.NotEmpty(t, collection.ID)

	dataobjects, err := fs.ListDataObjectsMasterReplica(conn, collection)
	assert.NoError(t, err)

	dataobjectPaths := []string{}

	for _, dataobject := range dataobjects {
		dataobjectPaths = append(dataobjectPaths, dataobject.Path)
		assert.NotEmpty(t, dataobject.ID)

		assert.Equal(t, string(types.GENERIC_DT), dataobject.DataType)

		assert.Equal(t, 1, len(dataobject.Replicas))
		for _, replica := range dataobject.Replicas {
			assert.NotEmpty(t, replica.Path)
			assert.NotEmpty(t, replica.ResourceName)
		}
	}

	assert.Equal(t, len(dataobjects), len(GetTestFiles()))
	assert.ElementsMatch(t, dataobjectPaths, GetTestFiles())
}

func testGetIRODSDataObject(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	homedir := getHomeDir(fsAPITestID)

	collection, err := fs.GetCollection(conn, homedir)
	assert.NoError(t, err)

	assert.Equal(t, homedir, collection.Path)
	assert.NotEmpty(t, collection.ID)

	for _, filepath := range GetTestFiles() {
		filename := path.Base(filepath)
		dirpath := path.Dir(filepath)
		assert.Equal(t, dirpath, homedir)

		dataobject, err := fs.GetDataObject(conn, collection, filename)
		assert.NoError(t, err)

		assert.NotEmpty(t, dataobject.ID)

		assert.Equal(t, string(types.GENERIC_DT), dataobject.DataType)

		for _, replica := range dataobject.Replicas {
			assert.NotEmpty(t, replica.Path)
		}
	}
}

func testGetIRODSDataObjectMasterReplica(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	homedir := getHomeDir(fsAPITestID)

	collection, err := fs.GetCollection(conn, homedir)
	assert.NoError(t, err)

	assert.Equal(t, homedir, collection.Path)
	assert.NotEmpty(t, collection.ID)

	for _, filepath := range GetTestFiles() {
		filename := path.Base(filepath)
		dirpath := path.Dir(filepath)
		assert.Equal(t, dirpath, homedir)

		dataobject, err := fs.GetDataObjectMasterReplica(conn, collection, filename)
		assert.NoError(t, err)

		assert.NotEmpty(t, dataobject.ID)

		assert.Equal(t, string(types.GENERIC_DT), dataobject.DataType)

		assert.Equal(t, 1, len(dataobject.Replicas))
		for _, replica := range dataobject.Replicas {
			assert.NotEmpty(t, replica.Path)
		}
	}
}

func testListIRODSDataObjectMeta(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	homedir := getHomeDir(fsAPITestID)

	collection, err := fs.GetCollection(conn, homedir)
	assert.NoError(t, err)
	assert.NotEmpty(t, collection.ID)

	for _, filepath := range GetTestFiles() {
		filename := path.Base(filepath)
		dirpath := path.Dir(filepath)
		assert.Equal(t, dirpath, homedir)

		metas, err := fs.ListDataObjectMeta(conn, collection, filename)
		assert.NoError(t, err)

		for _, meta := range metas {
			assert.NotEmpty(t, meta.AVUID)
		}
	}
}

func testListIRODSDataObjectAccess(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	homedir := getHomeDir(fsAPITestID)

	collection, err := fs.GetCollection(conn, homedir)
	assert.NoError(t, err)
	assert.NotEmpty(t, collection.ID)

	for _, filepath := range GetTestFiles() {
		filename := path.Base(filepath)
		dirpath := path.Dir(filepath)
		assert.Equal(t, dirpath, homedir)

		accesses, err := fs.ListDataObjectAccesses(conn, collection, filename)
		assert.NoError(t, err)

		for _, access := range accesses {
			assert.NotEmpty(t, access.Path)
			assert.Equal(t, filepath, access.Path)
		}
	}
}

func testCreateDeleteIRODSCollection(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	homedir := getHomeDir(fsAPITestID)

	// create
	newCollectionPath := homedir + "/testdir_" + xid.New().String()

	err = fs.CreateCollection(conn, newCollectionPath, true)
	assert.NoError(t, err)

	collection, err := fs.GetCollection(conn, newCollectionPath)
	assert.NoError(t, err)

	assert.Equal(t, newCollectionPath, collection.Path)
	assert.NotEmpty(t, collection.ID)

	// delete
	err = fs.DeleteCollection(conn, newCollectionPath, true, false)
	assert.NoError(t, err)

	_, err = fs.GetCollection(conn, newCollectionPath)
	deleted := false
	if err != nil {
		if types.IsFileNotFoundError(err) {
			// Okay!
			deleted = true
		}
	}

	assert.True(t, deleted)
}

func testCreateMoveDeleteIRODSCollection(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	homedir := getHomeDir(fsAPITestID)

	// create
	newCollectionPath := homedir + "/testdir_" + xid.New().String()

	err = fs.CreateCollection(conn, newCollectionPath, true)
	assert.NoError(t, err)

	collection, err := fs.GetCollection(conn, newCollectionPath)
	assert.NoError(t, err)

	assert.Equal(t, newCollectionPath, collection.Path)
	assert.NotEmpty(t, collection.ID)

	// move
	new2CollectionPath := homedir + "/testdir_" + xid.New().String()
	err = fs.MoveCollection(conn, newCollectionPath, new2CollectionPath)
	assert.NoError(t, err)

	new2Collection, err := fs.GetCollection(conn, new2CollectionPath)
	assert.NoError(t, err)

	assert.Equal(t, new2CollectionPath, new2Collection.Path)
	assert.NotEmpty(t, new2Collection.ID)

	// delete
	err = fs.DeleteCollection(conn, new2CollectionPath, true, false)
	assert.NoError(t, err)

	_, err = fs.GetCollection(conn, new2CollectionPath)
	deleted := false
	if err != nil {
		if types.IsFileNotFoundError(err) {
			// Okay!
			deleted = true
		}
	}

	assert.True(t, deleted)
}

func testCreateDeleteIRODSDataObject(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	homedir := getHomeDir(fsAPITestID)

	// create
	newDataObjectFilename := "testobj_" + xid.New().String()
	newDataObjectPath := homedir + "/" + newDataObjectFilename
	handle, err := fs.CreateDataObject(conn, newDataObjectPath, "", "w", true)
	assert.NoError(t, err)

	err = fs.CloseDataObject(conn, handle)
	assert.NoError(t, err)

	collection, err := fs.GetCollection(conn, homedir)
	assert.NoError(t, err)

	obj, err := fs.GetDataObject(conn, collection, newDataObjectFilename)
	assert.NoError(t, err)
	assert.NotEmpty(t, obj.ID)

	// delete
	err = fs.DeleteDataObject(conn, newDataObjectPath, true)
	assert.NoError(t, err)

	_, err = fs.GetDataObject(conn, collection, newDataObjectFilename)
	deleted := false
	if err != nil {
		if types.IsFileNotFoundError(err) {
			// Okay!
			deleted = true
		}
	}

	assert.True(t, deleted)
}

func testReadWriteIRODSDataObject(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	homedir := getHomeDir(fsAPITestID)

	// create
	newDataObjectFilename := "testobj_" + xid.New().String()
	newDataObjectPath := homedir + "/" + newDataObjectFilename

	handle, err := fs.CreateDataObject(conn, newDataObjectPath, "", "w", true)
	assert.NoError(t, err)

	data := "Hello World"
	err = fs.WriteDataObject(conn, handle, []byte(data))
	assert.NoError(t, err)

	err = fs.CloseDataObject(conn, handle)
	assert.NoError(t, err)

	collection, err := fs.GetCollection(conn, homedir)
	assert.NoError(t, err)

	obj, err := fs.GetDataObject(conn, collection, newDataObjectFilename)
	assert.NoError(t, err)
	assert.NotEmpty(t, obj.ID)

	// read
	handle, _, err = fs.OpenDataObject(conn, newDataObjectPath, "", "r")
	assert.NoError(t, err)

	buf := make([]byte, len(data))
	recvLen, err := fs.ReadDataObject(conn, handle, buf)
	assert.NoError(t, err)

	assert.Equal(t, len(data), recvLen)
	assert.Equal(t, data, string(buf))

	err = fs.CloseDataObject(conn, handle)
	assert.NoError(t, err)

	// delete
	err = fs.DeleteDataObject(conn, newDataObjectPath, true)
	assert.NoError(t, err)
}

func testReadWriteIRODSDataObjectWithSingleConnection(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	homedir := getHomeDir(fsAPITestID)

	// create
	newDataObjectFilename := "testobj_" + xid.New().String()
	newDataObjectPath := homedir + "/" + newDataObjectFilename

	handle, err := fs.CreateDataObject(conn, newDataObjectPath, "", "w", true)
	assert.NoError(t, err)

	data := "Hello World"
	err = fs.WriteDataObject(conn, handle, []byte(data))
	assert.NoError(t, err)

	err = fs.CloseDataObject(conn, handle)
	assert.NoError(t, err)

	collection, err := fs.GetCollection(conn, homedir)
	assert.NoError(t, err)

	obj, err := fs.GetDataObject(conn, collection, newDataObjectFilename)
	assert.NoError(t, err)
	assert.NotEmpty(t, obj.ID)

	// read 1
	handle1, _, err := fs.OpenDataObject(conn, newDataObjectPath, "", "r")
	assert.NoError(t, err)

	// read 2
	handle2, _, err := fs.OpenDataObject(conn, newDataObjectPath, "", "r")
	assert.NoError(t, err)

	buf1 := make([]byte, len(data))
	recvLen1, err := fs.ReadDataObject(conn, handle1, buf1[:5])
	assert.NoError(t, err)

	buf2 := make([]byte, len(data))
	recvLen2, err := fs.ReadDataObject(conn, handle2, buf2[:4])
	assert.NoError(t, err)

	recvLen3, err := fs.ReadDataObject(conn, handle1, buf1[5:])
	assert.NoError(t, err)

	recvLen4, err := fs.ReadDataObject(conn, handle2, buf2[4:])
	assert.NoError(t, err)

	err = fs.CloseDataObject(conn, handle1)
	assert.NoError(t, err)

	err = fs.CloseDataObject(conn, handle2)
	assert.NoError(t, err)

	assert.Equal(t, len(data), recvLen1+recvLen3)
	assert.Equal(t, len(data), recvLen2+recvLen4)
	assert.Equal(t, data, string(buf1))
	assert.Equal(t, data, string(buf2))

	// delete
	err = fs.DeleteDataObject(conn, newDataObjectPath, true)
	assert.NoError(t, err)
}

func testMixedReadWriteIRODSDataObjectWithSingleConnection(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	homedir := getHomeDir(fsAPITestID)

	// create a new side file
	newSideDataObjectFilename := "testobj_" + xid.New().String()
	newSideDataObjectPath := homedir + "/" + newSideDataObjectFilename

	handleSide, err := fs.CreateDataObject(conn, newSideDataObjectPath, "", "w", true)
	assert.NoError(t, err)

	// create
	newDataObjectFilename := "testobj_" + xid.New().String()
	newDataObjectPath := homedir + "/" + newDataObjectFilename

	handle, err := fs.CreateDataObject(conn, newDataObjectPath, "", "w", true)
	assert.NoError(t, err)

	data := "Hello World"
	err = fs.WriteDataObject(conn, handle, []byte(data))
	assert.NoError(t, err)

	err = fs.CloseDataObject(conn, handle)
	assert.NoError(t, err)

	collection, err := fs.GetCollection(conn, homedir)
	assert.NoError(t, err)

	obj, err := fs.GetDataObject(conn, collection, newDataObjectFilename)
	assert.NoError(t, err)
	assert.NotEmpty(t, obj.ID)

	// read 1
	handle1, _, err := fs.OpenDataObject(conn, newDataObjectPath, "", "r")
	assert.NoError(t, err)

	// read 2
	handle2, _, err := fs.OpenDataObject(conn, newDataObjectPath, "", "r")
	assert.NoError(t, err)

	// write to side file
	err = fs.WriteDataObject(conn, handleSide, []byte(data))
	assert.NoError(t, err)

	err = fs.CloseDataObject(conn, handleSide)
	assert.NoError(t, err)

	// rollback test
	// this causes operation error, rollbacks the transaction
	conn.Lock()
	err = conn.PoorMansRollback()
	conn.Unlock()
	assert.NoError(t, err)

	// continue reading
	buf1 := make([]byte, len(data))
	recvLen1, err := fs.ReadDataObject(conn, handle1, buf1[:5])
	assert.NoError(t, err)

	buf2 := make([]byte, len(data))
	recvLen2, err := fs.ReadDataObject(conn, handle2, buf2[:4])
	assert.NoError(t, err)

	recvLen3, err := fs.ReadDataObject(conn, handle1, buf1[5:])
	assert.NoError(t, err)

	err = fs.CloseDataObject(conn, handle1)
	assert.NoError(t, err)

	recvLen4, err := fs.ReadDataObject(conn, handle2, buf2[4:])
	assert.NoError(t, err)

	err = fs.CloseDataObject(conn, handle2)
	assert.NoError(t, err)

	assert.Equal(t, len(data), recvLen1+recvLen3)
	assert.Equal(t, len(data), recvLen2+recvLen4)
	assert.Equal(t, data, string(buf1))
	assert.Equal(t, data, string(buf2))

	// delete
	err = fs.DeleteDataObject(conn, newDataObjectPath, true)
	assert.NoError(t, err)
}

func testTruncateIRODSDataObject(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	homedir := getHomeDir(fsAPITestID)

	// create
	newDataObjectFilename := "testobj_" + xid.New().String()
	newDataObjectPath := homedir + "/" + newDataObjectFilename

	handle, err := fs.CreateDataObject(conn, newDataObjectPath, "", "w", true)
	assert.NoError(t, err)

	data := "Hello World Test!!!!"
	err = fs.WriteDataObject(conn, handle, []byte(data))
	assert.NoError(t, err)

	err = fs.TruncateDataObjectHandle(conn, handle, 11)
	assert.NoError(t, err)

	err = fs.CloseDataObject(conn, handle)
	assert.NoError(t, err)

	collection, err := fs.GetCollection(conn, homedir)
	assert.NoError(t, err)

	obj, err := fs.GetDataObject(conn, collection, newDataObjectFilename)
	assert.NoError(t, err)
	assert.NotEmpty(t, obj.ID)

	// read
	handle, _, err = fs.OpenDataObject(conn, newDataObjectPath, "", "r")
	assert.NoError(t, err)

	buf := make([]byte, len(data))
	recvLen, err := fs.ReadDataObject(conn, handle, buf)
	assert.Equal(t, io.EOF, err)

	assert.Equal(t, 11, recvLen)
	assert.Equal(t, "Hello World", string(buf[:recvLen]))

	err = fs.CloseDataObject(conn, handle)
	assert.NoError(t, err)

	// delete
	err = fs.DeleteDataObject(conn, newDataObjectPath, true)
	assert.NoError(t, err)
}

func testListIRODSGroupUsers(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	users, err := fs.ListGroupUsers(conn, "rodsadmin")
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(users), 1)

	adminUsers := []string{}
	for _, user := range users {
		assert.NotEmpty(t, user.ID)

		adminUsers = append(adminUsers, user.Name)
	}

	assert.Contains(t, adminUsers, "rods")
}

func testSearchDataObjectsByMeta(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	for _, testFilePath := range GetTestFiles() {
		sha1sum := sha1.New()
		_, err = sha1sum.Write([]byte(testFilePath))
		assert.NoError(t, err)

		hashBytes := sha1sum.Sum(nil)
		hashString := hex.EncodeToString(hashBytes)

		dataobjects, err := fs.SearchDataObjectsByMeta(conn, "hash", hashString)
		assert.NoError(t, err)

		assert.Equal(t, 1, len(dataobjects))
		assert.Equal(t, testFilePath, dataobjects[0].Path)
	}
}

func testSearchDataObjectsByMetaWildcard(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	conn := connection.NewIRODSConnection(account, 300*time.Second, "go-irodsclient-test")
	err := conn.Connect()
	assert.NoError(t, err)
	defer conn.Disconnect()

	// this takes a long time to perform
	for _, testFilePath := range GetTestFiles() {
		sha1sum := sha1.New()
		_, err = sha1sum.Write([]byte(testFilePath))
		assert.NoError(t, err)

		hashBytes := sha1sum.Sum(nil)
		hashString := hex.EncodeToString(hashBytes)

		dataobjects, err := fs.SearchDataObjectsByMetaWildcard(conn, "hash", hashString+"%")
		assert.NoError(t, err)

		assert.Equal(t, 1, len(dataobjects))
		assert.Equal(t, testFilePath, dataobjects[0].Path)
	}

	dataobjects, err := fs.SearchDataObjectsByMetaWildcard(conn, "tag", "test%")
	assert.NoError(t, err)

	assert.Equal(t, len(GetTestFiles()), len(dataobjects))
}

func testParallelUploadAndDownloadDataObject(t *testing.T) {
	account := GetTestAccount()

	account.ClientServerNegotiation = false

	sessionConfig := session.NewIRODSSessionConfigWithDefault("go-irodsclient-test")

	sess, err := session.NewIRODSSession(account, sessionConfig)
	assert.NoError(t, err)
	defer sess.Release()

	conn, err := sess.AcquireConnection()
	assert.NoError(t, err)

	if !fs.SupportParallUpload(conn) {
		sess.ReturnConnection(conn)
		return
	}

	homedir := getHomeDir(fsAPITestID)

	// gen very large file
	testval := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" // 62
	fileSize := 100 * 1024 * 1024                                               // 100MB

	filename := "test_large_file.bin"
	bufSize := 1024
	buf := make([]byte, bufSize)

	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	assert.NoError(t, err)

	for i := 0; i < fileSize/bufSize; i++ {
		// fill buf
		for j := 0; j < bufSize; j++ {
			buf[j] = testval[j%len(testval)]
		}

		_, err = f.Write(buf)
		assert.NoError(t, err)
	}

	err = f.Close()
	assert.NoError(t, err)

	// upload
	irodsPath := homedir + "/" + filename
	err = fs.UploadDataObjectParallel(sess, filename, irodsPath, "", 4, false, nil)
	assert.NoError(t, err)

	err = os.Remove(filename)
	assert.NoError(t, err)

	// get
	collection, err := fs.GetCollection(conn, homedir)
	assert.NoError(t, err)

	obj, err := fs.GetDataObject(conn, collection, filename)
	assert.NoError(t, err)
	assert.NotEmpty(t, obj.ID)
	assert.Equal(t, int64(fileSize), obj.Size)

	// delete
	err = fs.DeleteDataObject(conn, irodsPath, true)
	assert.NoError(t, err)

	sess.ReturnConnection(conn)
}
