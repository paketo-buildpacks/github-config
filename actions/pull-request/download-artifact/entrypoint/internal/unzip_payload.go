package internal

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

func GetArtifactZip(archiveDownloadURL, token string) ([]byte, error) {
	var bearer = "Bearer " + token
	req, err := http.NewRequest("GET", archiveDownloadURL, nil)
	req.Header.Add("Authorization", bearer)
	payloadResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to get artifact zip file: %w", err)
	}

	defer payloadResp.Body.Close()
	payloadBody, err := io.ReadAll(payloadResp.Body)
	if err != nil {
		return []byte{}, err
	}

	return payloadBody, nil
}

func UnzipPayload(payloadName string, payloadBody []byte, zipSize int) ([]byte, error) {
	zipReader, err := zip.NewReader(bytes.NewReader(payloadBody), int64(zipSize))
	if err != nil {
		return []byte{}, err
	}

	for _, zipFile := range zipReader.File {
		fmt.Println("Reading file:", zipFile.Name)

		if zipFile.Name == payloadName {
			unzippedFileBytes, err := readZipFile(zipFile)
			if err != nil {
				return []byte{}, err
			}
			return unzippedFileBytes, nil
		}
	}
	return []byte{}, fmt.Errorf("no payload with the name %s found in zip", payloadName)
}

func readZipFile(zf *zip.File) ([]byte, error) {
	f, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ioutil.ReadAll(f)
}
