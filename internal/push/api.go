// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package push

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/lirios/ostree-upload/internal/common"
	"github.com/lirios/ostree-upload/internal/logger"
)

// Client is used to upload objects to a receiver
type Client struct {
	url        *url.URL
	userAgent  string
	httpClient *http.Client
	token      string
}

// NewClient creates a new upload client connecting to the specified receiver endpoint
func NewClient(endpoint, token string) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		DisableCompression: false,
	}
	httpClient := &http.Client{Transport: transport, Timeout: 60 * time.Second}

	return &Client{u, "ostree-upload", httpClient, token}, nil
}

func (c *Client) newRequest(method, path string, body interface{}) (*http.Request, error) {
	rel := &url.URL{Path: path}
	u := c.url.ResolveReference(rel)

	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}

	request, err := http.NewRequest(method, u.String(), buf)
	if err != nil {
		return nil, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", c.userAgent)
	request.Header.Set("Authorization", fmt.Sprintf("BEARER %s", c.token))
	return request, nil
}

func (c *Client) do(request *http.Request, v interface{}) (*http.Response, error) {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		logger.Errorf("Cannot parse response: %v", err)
		return response, err
	}

	bodyString := strings.TrimSuffix(string(body), "\n")

	if response.StatusCode != http.StatusOK {
		return response, errors.New(bodyString)
	}

	if v != nil {
		err = json.Unmarshal(body, v)
		if err != nil {
			logger.Errorf("Error decoding response: %v", err)
			if e, ok := err.(*json.SyntaxError); ok {
				logger.Errorf("Syntax error at byte offset %d", e.Offset)
			}
			logger.Infof("Response: %q", body)
			return nil, err
		}
	}

	return response, err
}

// GetInfo retries remote repository information
func (c *Client) GetInfo() (*common.InfoResponse, error) {
	request, err := c.newRequest("GET", "/api/v1/info", nil)
	if err != nil {
		return nil, err
	}

	var info common.InfoResponse
	_, err = c.do(request, &info)
	if err != nil {
		return nil, err
	}

	return &info, err
}

// NewQueueEntry tells the server which branches need to be updated
func (c *Client) NewQueueEntry(updateRefs map[string]common.RevisionPair, objects []string) (string, error) {
	req := common.QueueRequest{Refs: updateRefs, Objects: objects}
	request, err := c.newRequest("POST", "/api/v1/queue", req)
	if err != nil {
		return "", err
	}

	var result common.UpdateResponse
	_, err = c.do(request, &result)
	if err != nil {
		return "", err
	}

	return result.QueueID, nil
}

// DeleteQueueEntry removes the entry from the queue
func (c *Client) DeleteQueueEntry(queueID string) error {
	request, err := c.newRequest("DELETE", fmt.Sprintf("/api/v1/queue/%s", queueID), nil)
	if err != nil {
		return err
	}

	_, err = c.do(request, nil)
	if err != nil {
		return err
	}

	return nil
}

// SendObjectsList sends the list of missing objects to the server which will reply
// with the list of objects that were not already submitted by a previous upload
func (c *Client) SendObjectsList(queueID string) ([]string, error) {
	request, err := c.newRequest("GET", fmt.Sprintf("/api/v1/queue/%s", queueID), nil)
	if err != nil {
		return nil, err
	}

	var result common.ObjectsResponse
	_, err = c.do(request, &result)
	if err != nil {
		return nil, err
	}

	return result.Objects, nil
}

// Upload uploads an object
func (c *Client) Upload(queueID string, object *common.Object) error {
	file, err := os.Open(object.ObjectPath)
	if err != nil {
		return err
	}
	defer file.Close()

	body := &bytes.Buffer{}

	writer := multipart.NewWriter(body)

	if err := writer.WriteField("rev", object.Rev); err != nil {
		return err
	}
	if err := writer.WriteField("object_name", object.ObjectName); err != nil {
		return err
	}
	if err := writer.WriteField("checksum", object.Checksum); err != nil {
		return err
	}

	part, err := writer.CreateFormFile("file", object.ObjectName)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	rel := &url.URL{Path: fmt.Sprintf("/api/v1/queue/%s", queueID)}
	u := c.url.ResolveReference(rel)

	request, err := http.NewRequest("PUT", u.String(), body)
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", c.userAgent)
	request.Header.Set("Authorization", fmt.Sprintf("BEARER %s", c.token))

	if _, err := c.httpClient.Do(request); err != nil {
		return err
	}

	// Wait for the connection to be closed to avoid the "too many open files"
	// error on the server
	time.Sleep(15 * time.Second)

	return nil
}

// Done tells the server that the upload is done
func (c *Client) Done(queueID string) error {
	request, err := c.newRequest("GET", fmt.Sprintf("/api/v1/done/%s", queueID), nil)
	if err != nil {
		return err
	}

	_, err = c.do(request, nil)
	if err != nil {
		return err
	}

	return nil
}
