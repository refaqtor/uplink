// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package testsuite_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/paths"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/uplink"
	privateAccess "storj.io/uplink/private/access"
)

func TestSharePermisions(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		apiKey := planet.Uplinks[0].APIKey[satellite.ID()]
		uplinkConfig := uplink.Config{}
		access, err := uplinkConfig.RequestAccessWithPassphrase(ctx, satellite.URL().String(), apiKey.Serialize(), "mypassphrase")
		require.NoError(t, err)

		items := []struct {
			AllowDownload bool
			AllowUpload   bool
			AllowList     bool
			AllowDelete   bool
		}{
			{false, false, false, false},
			{true, true, true, true},

			{true, false, false, false},
			{false, true, false, false},
			{false, false, true, false},
			{false, false, false, true},

			// TODO generate all combinations automatically
		}

		expectedData := testrand.Bytes(10 * memory.KiB)
		{
			project := openProject(t, ctx, planet)
			require.NoError(t, err)

			// prepare bucket and object for all test cases
			for i := range items {
				bucketName := "testbucket" + strconv.Itoa(i)
				bucket, err := project.EnsureBucket(ctx, bucketName)
				require.NoError(t, err)
				require.NotNil(t, bucket)
				require.Equal(t, bucketName, bucket.Name)

				upload, err := project.UploadObject(ctx, bucketName, "test.dat", nil)
				require.NoError(t, err)

				source := bytes.NewBuffer(expectedData)
				_, err = io.Copy(upload, source)
				require.NoError(t, err)

				err = upload.Commit()
				require.NoError(t, err)
			}

			ctx.Check(project.Close)
		}

		for i, item := range items {
			i := i
			item := item

			name := func() string {
				result := make([]string, 0, 4)
				if item.AllowDownload {
					result = append(result, "AllowDownload")
				}
				if item.AllowUpload {
					result = append(result, "AllowUpload")
				}
				if item.AllowDelete {
					result = append(result, "AllowDelete")
				}
				if item.AllowList {
					result = append(result, "AllowList")
				}
				return strings.Join(result, "_")
			}

			t.Run(name(), func(t *testing.T) {
				permission := uplink.Permission{
					AllowDownload: item.AllowDownload,
					AllowUpload:   item.AllowUpload,
					AllowDelete:   item.AllowDelete,
					AllowList:     item.AllowList,
				}
				sharedAccess, err := access.Share(permission)
				if permission == (uplink.Permission{}) {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)

				project, err := uplinkConfig.OpenProject(ctx, sharedAccess)
				require.NoError(t, err)

				defer ctx.Check(project.Close)

				bucketName := "testbucket" + strconv.Itoa(i)
				{ // reading
					download, err := project.DownloadObject(ctx, bucketName, "test.dat", nil)
					if item.AllowDownload {
						require.NoError(t, err)

						var downloaded bytes.Buffer
						_, err = io.Copy(&downloaded, download)

						require.NoError(t, err)
						require.Equal(t, expectedData, downloaded.Bytes())

						err = download.Close()
						require.NoError(t, err)
					} else {
						require.Error(t, err)
					}
				}
				{ // writing
					upload, err := project.UploadObject(ctx, bucketName, "new-test.dat", nil)
					require.NoError(t, err)

					source := bytes.NewBuffer(expectedData)
					_, err = io.Copy(upload, source)
					if item.AllowUpload {
						require.NoError(t, err)

						err = upload.Commit()
						require.NoError(t, err)
					} else {
						require.Error(t, err)
					}
				}
				{ // deleting
					// TODO test removing object
					// _, err := project.DeleteBucket(ctx, bucketName)
					// if item.AllowDelete {
					// 	require.NoError(t, err)
					// } else {
					// 	require.Error(t, err)
					// }
				}

				// TODO test listing buckets and objects

			})
		}
	})
}

func TestSharePermisionsNotAfterNotBefore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		apiKey := planet.Uplinks[0].APIKey[satellite.ID()]
		uplinkConfig := uplink.Config{}
		access, err := uplinkConfig.RequestAccessWithPassphrase(ctx, satellite.URL().String(), apiKey.Serialize(), "mypassphrase")
		require.NoError(t, err)

		{ // error when Before is earlier then After
			permission := uplink.FullPermission()
			permission.NotBefore = time.Now()
			permission.NotAfter = permission.NotBefore.Add(-1 * time.Hour)
			_, err := access.Share(permission)
			require.Error(t, err)
		}
		{ // don't permit operations until one hour from now
			permission := uplink.FullPermission()
			permission.NotBefore = time.Now().Add(time.Hour)
			sharedAccess, err := access.Share(permission)
			require.NoError(t, err)

			project, err := uplink.OpenProject(ctx, sharedAccess)
			require.NoError(t, err)

			bucket, err := project.EnsureBucket(ctx, "test-bucket")
			require.Error(t, err)
			require.Nil(t, bucket)
		}
	})
}

func TestAccessSerialization(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		apiKey := planet.Uplinks[0].APIKey[satellite.ID()]

		access, err := uplink.RequestAccessWithPassphrase(ctx, satellite.URL().String(), apiKey.Serialize(), "mypassphrase")
		require.NoError(t, err)

		// try to serialize and deserialize access and use it for upload/download
		serializedAccess, err := access.Serialize()
		require.NoError(t, err)

		access, err = uplink.ParseAccess(serializedAccess)
		require.NoError(t, err)

		project, err := uplink.OpenProject(ctx, access)
		require.NoError(t, err)

		defer ctx.Check(project.Close)

		bucket, err := project.EnsureBucket(ctx, "test-bucket")
		require.NoError(t, err)
		require.NotNil(t, bucket)
		require.Equal(t, "test-bucket", bucket.Name)

		upload, err := project.UploadObject(ctx, "test-bucket", "test.dat", nil)
		require.NoError(t, err)
		assertObjectEmptyCreated(t, upload.Info(), "test.dat")

		randData := testrand.Bytes(1 * memory.KiB)
		source := bytes.NewBuffer(randData)
		_, err = io.Copy(upload, source)
		require.NoError(t, err)
		assertObjectEmptyCreated(t, upload.Info(), "test.dat")

		err = upload.Commit()
		require.NoError(t, err)
		assertObject(t, upload.Info(), "test.dat")

		err = upload.Commit()
		require.True(t, errors.Is(err, uplink.ErrUploadDone))

		download, err := project.DownloadObject(ctx, "test-bucket", "test.dat", nil)
		require.NoError(t, err)
		assertObject(t, download.Info(), "test.dat")
	})
}

func TestUploadNotAllowedPath(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		apiKey := planet.Uplinks[0].APIKey[satellite.ID()]
		access, err := uplink.RequestAccessWithPassphrase(ctx, satellite.URL().String(), apiKey.Serialize(), "mypassphrase")
		require.NoError(t, err)

		err = planet.Uplinks[0].CreateBucket(ctx, satellite, "testbucket")
		require.NoError(t, err)

		sharedAccess, err := access.Share(uplink.FullPermission(), uplink.SharePrefix{
			Bucket: "testbucket",
			Prefix: "videos",
		})
		require.NoError(t, err)

		project, err := uplink.OpenProject(ctx, sharedAccess)
		require.NoError(t, err)
		defer ctx.Check(project.Close)

		testData := bytes.NewBuffer(testrand.Bytes(1 * memory.KiB))

		upload, err := project.UploadObject(ctx, "testbucket", "first-level-object", nil)
		require.NoError(t, err)

		_, err = io.Copy(upload, testData)
		require.Error(t, err)

		err = upload.Abort()
		require.NoError(t, err)

		upload, err = project.UploadObject(ctx, "testbucket", "videos/second-level-object", nil)
		require.NoError(t, err)

		_, err = io.Copy(upload, testData)
		require.NoError(t, err)

		err = upload.Commit()
		require.NoError(t, err)
	})
}

func TestListObjects_EncryptionBypass(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		apiKey := planet.Uplinks[0].APIKey[satellite.ID()]
		access, err := uplink.RequestAccessWithPassphrase(ctx, satellite.URL().String(), apiKey.Serialize(), "mypassphrase")
		require.NoError(t, err)

		bucketName := "testbucket"
		err = planet.Uplinks[0].CreateBucket(ctx, satellite, bucketName)
		require.NoError(t, err)

		filePaths := []string{
			"a", "aa", "b", "bb", "c",
			"a/xa", "a/xaa", "a/xb", "a/xbb", "a/xc",
			"b/ya", "b/yaa", "b/yb", "b/ybb", "b/yc",
		}

		for _, path := range filePaths {
			err = planet.Uplinks[0].Upload(ctx, satellite, bucketName, path, testrand.Bytes(memory.KiB))
			require.NoError(t, err)
		}
		sort.Strings(filePaths)

		// Enable encryption bypass
		err = privateAccess.EnablePathEncryptionBypass(access)
		require.NoError(t, err)

		project, err := uplink.OpenProject(ctx, access)
		require.NoError(t, err)

		objects := project.ListObjects(ctx, bucketName, &uplink.ListObjectsOptions{
			Recursive: true,
		})

		// TODO verify that decoded string can be decrypted to defined filePaths,
		// currently it's not possible because we have no access encryption access store.
		for objects.Next() {
			item := objects.Item()

			iter := paths.NewUnencrypted(item.Key).Iterator()
			for !iter.Done() {
				next := iter.Next()

				// verify that path segments are encoded with base64
				_, err = base64.URLEncoding.DecodeString(next)
				require.NoError(t, err)
			}
		}
		require.NoError(t, objects.Err())
	})
}

func TestDeleteObject_EncryptionBypass(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		apiKey := planet.Uplinks[0].APIKey[satellite.ID()]
		access, err := uplink.RequestAccessWithPassphrase(ctx, satellite.URL().String(), apiKey.Serialize(), "mypassphrase")
		require.NoError(t, err)

		bucketName := "testbucket"
		err = planet.Uplinks[0].CreateBucket(ctx, satellite, bucketName)
		require.NoError(t, err)

		err = planet.Uplinks[0].Upload(ctx, satellite, bucketName, "test-file", testrand.Bytes(memory.KiB))
		require.NoError(t, err)

		err = privateAccess.EnablePathEncryptionBypass(access)
		require.NoError(t, err)

		project, err := uplink.OpenProject(ctx, access)
		require.NoError(t, err)

		objects := project.ListObjects(ctx, bucketName, &uplink.ListObjectsOptions{
			Recursive: true,
		})

		for objects.Next() {
			item := objects.Item()

			_, err = base64.URLEncoding.DecodeString(item.Key)
			require.NoError(t, err)

			_, err = project.DeleteObject(ctx, bucketName, item.Key)
			require.NoError(t, err)
		}
		require.NoError(t, objects.Err())

		// this means that object was deleted and empty bucket can be deleted
		_, err = project.DeleteBucket(ctx, bucketName)
		require.NoError(t, err)
	})
}