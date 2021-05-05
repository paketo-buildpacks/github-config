package internal

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

func GetArtifactZip(archiveDownloadURL, token string) (io.ReadCloser, error) {
	var bearer = "Bearer " + token
	req, err := http.NewRequest("GET", archiveDownloadURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", bearer)
	payloadResp, err := http.DefaultClient.Do(req)
	if err != nil {
		payloadResp.Body.Close()
		return nil, fmt.Errorf("failed to get artifact zip file: %w", err)
	}

	if payloadResp.StatusCode != http.StatusOK {
		payloadResp.Body.Close()
		return nil, fmt.Errorf("failed getting payload with status code: %d", payloadResp.StatusCode)
	}

	return payloadResp.Body, nil
}

func UnzipPayload(payloadName string, payloadResponseBody io.ReadCloser, zipSize int) ([]byte, error) {
	defer payloadResponseBody.Close()
	payloadBody, err := io.ReadAll(payloadResponseBody)
	if err != nil {
		return nil, err
	}

	zipReader, err := zip.NewReader(bytes.NewReader(payloadBody), int64(zipSize))
	if err != nil {
		return nil, err
	}

	for _, zipFile := range zipReader.File {
		fmt.Println("Reading file:", zipFile.Name)

		if zipFile.Name == payloadName {
			unzippedFileBytes, err := readZipFile(zipFile)
			if err != nil {
				return nil, err
			}
			return unzippedFileBytes, nil
		}
	}
	return nil, fmt.Errorf("no payload with the name %s found in zip", payloadName)
}

func readZipFile(zf *zip.File) ([]byte, error) {
	f, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ioutil.ReadAll(f)
}
