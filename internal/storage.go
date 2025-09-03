package internal

import (
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var Storage *minio.Client

func StorageSetup() (err error) {
	// Initialize minio client object.
	Storage, err = minio.New(os.Getenv(""), &minio.Options{
		Creds:  credentials.NewStaticV4(os.Getenv(""), os.Getenv(""), ""),
		Secure: false,
	})

	if err != nil {
		return
	}

	return
}
