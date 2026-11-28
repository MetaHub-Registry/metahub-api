// Command s3probe exercises every S3 operation the registry relies on against a
// live object store. It exists so that a backend swap is a test rather than a
// claim: the registry talks to storage through nine calls, and a candidate
// backend is accepted only when all nine pass.
//
// The probe creates its own objects under the "probe/" prefix, verifies them,
// and removes them again. It does not touch the registry's buckets.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type step struct {
	name string
	err  error
}

func main() {
	var (
		endpoint = flag.String("endpoint", "localhost:9000", "S3 endpoint, host:port")
		access   = flag.String("access", os.Getenv("MINIO_ROOT_USER"), "access key (default $MINIO_ROOT_USER)")
		secret   = flag.String("secret", os.Getenv("MINIO_ROOT_PASSWORD"), "secret key (default $MINIO_ROOT_PASSWORD)")
		region   = flag.String("region", "", "region, if the backend requires one")
		bucket   = flag.String("bucket", "s3probe", "bucket to create and probe")
		secure   = flag.Bool("secure", false, "use TLS")
		mbSize   = flag.Int("multipart-mib", 20, "size of the multipart object in MiB")
		partSize = flag.Int("part-mib", 5, "multipart part size in MiB")
		timeout  = flag.Duration("timeout", 2*time.Minute, "overall timeout")
	)
	flag.Parse()

	if *access == "" || *secret == "" {
		fmt.Fprintln(os.Stderr, "s3probe: -access and -secret are required (or MINIO_ROOT_USER/MINIO_ROOT_PASSWORD)")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	cli, err := minio.New(*endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(*access, *secret, ""),
		Secure: *secure,
		Region: *region,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "s3probe: new client: %v\n", err)
		os.Exit(2)
	}

	var steps []*step
	add := func(s *step) { steps = append(steps, s) }

	// BucketExists, then MakeBucket only if it is missing.
	exists, err := cli.BucketExists(ctx, *bucket)
	add(&step{"BucketExists", err})
	if err == nil && !exists {
		add(&step{"MakeBucket", cli.MakeBucket(ctx, *bucket, minio.MakeBucketOptions{Region: *region})})
	} else {
		add(&step{"MakeBucket", nil}) // bucket was already there; nothing to create
	}

	const (
		plainKey = "probe/plain.txt"
		partKey  = "probe/multipart.bin"
		single   = "probe/single.txt"
	)
	plainBody := []byte("s3probe plain object\n")
	partBody := make([]byte, *mbSize<<20)
	for i := range partBody {
		partBody[i] = byte(i * 31)
	}

	put := func(key string, body []byte, opts minio.PutObjectOptions) *step {
		_, err := cli.PutObject(ctx, *bucket, key, bytes.NewReader(body), int64(len(body)), opts)
		return &step{"PutObject " + key, err}
	}
	add(put(plainKey, plainBody, minio.PutObjectOptions{ContentType: "text/plain"}))
	add(put(partKey, partBody, minio.PutObjectOptions{PartSize: uint64(*partSize) << 20}))
	add(put(single, []byte("remove me\n"), minio.PutObjectOptions{}))

	var statErr error
	if _, err := cli.StatObject(ctx, *bucket, plainKey, minio.StatObjectOptions{}); err != nil {
		statErr = err
	}
	add(&step{"StatObject", statErr})

	get := func(key string, want []byte) *step {
		obj, err := cli.GetObject(ctx, *bucket, key, minio.GetObjectOptions{})
		if err != nil {
			return &step{"GetObject " + key, err}
		}
		defer obj.Close()
		got, err := io.ReadAll(obj)
		if err != nil {
			return &step{"GetObject " + key, err}
		}
		if !bytes.Equal(got, want) {
			return &step{"GetObject " + key, fmt.Errorf("content mismatch: %d bytes read, %d expected", len(got), len(want))}
		}
		return &step{"GetObject " + key, nil}
	}
	add(get(plainKey, plainBody))
	add(get(partKey, partBody))

	// PresignedGetObject: the URL must be usable by a client without credentials.
	presignErr := func() error {
		u, err := cli.PresignedGetObject(ctx, *bucket, plainKey, time.Hour, nil)
		if err != nil {
			return err
		}
		resp, err := http.Get(u.String())
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("GET %s: %s", u.Path, resp.Status)
		}
		if !bytes.Equal(body, plainBody) {
			return fmt.Errorf("presigned content mismatch")
		}
		return nil
	}()
	add(&step{"PresignedGetObject", presignErr})

	// ListObjects: every probed key must be visible under the prefix.
	listErr := func() error {
		var keys []string
		for obj := range cli.ListObjects(ctx, *bucket, minio.ListObjectsOptions{Prefix: "probe/", Recursive: true}) {
			if obj.Err != nil {
				return obj.Err
			}
			keys = append(keys, obj.Key)
		}
		sort.Strings(keys)
		want := []string{partKey, plainKey, single}
		if strings.Join(keys, ",") != strings.Join(want, ",") {
			return fmt.Errorf("listed %v, expected %v", keys, want)
		}
		return nil
	}()
	add(&step{"ListObjects", listErr})

	add(&step{"RemoveObject", cli.RemoveObject(ctx, *bucket, single, minio.RemoveObjectOptions{})})

	// RemoveObjects (bulk): drain the error channel until it closes.
	bulkErr := func() error {
		ch := make(chan minio.ObjectInfo, 2)
		ch <- minio.ObjectInfo{Key: plainKey}
		ch <- minio.ObjectInfo{Key: partKey}
		close(ch)
		for rmErr := range cli.RemoveObjects(ctx, *bucket, ch, minio.RemoveObjectsOptions{}) {
			if rmErr.Err != nil {
				return rmErr.Err
			}
		}
		return nil
	}()
	add(&step{"RemoveObjects", bulkErr})

	failed := 0
	for _, s := range steps {
		if s.err != nil {
			failed++
			fmt.Printf("FAIL  %-28s %v\n", s.name, s.err)
			continue
		}
		fmt.Printf("ok    %-28s\n", s.name)
	}
	fmt.Printf("\n%d checks, %d failed\n", len(steps), failed)
	if failed > 0 {
		os.Exit(1)
	}
}
