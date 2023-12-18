package pkg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"time"

	//"github.com/mss-boot-io/mss-boot/proto"
	"golang.org/x/net/websocket"
	"google.golang.org/grpc/metadata"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2023/12/5 18:39:41
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2023/12/5 18:39:41
 */

type Task struct {
	ID       string
	Name     string
	Endpoint string
	Method   string
	Command  string
	Args     []string
	Python   string
	Writer   io.Writer
	Metadata map[string]string
	Timeout  time.Duration
}

func (t *Task) Run() error {
	ctx := metadata.AppendToOutgoingContext(context.TODO(), "taskID", t.ID)
	for k, v := range t.Metadata {
		ctx = metadata.AppendToOutgoingContext(ctx, k, v)
	}
	ctx, cancel := context.WithTimeout(ctx, t.Timeout)
	defer cancel()
	// script
	if t.Endpoint == "" {
		//exec script
		cmd := exec.Command(t.Command, t.Args...)
		cmd.WaitDelay = t.Timeout
		cmd.Stdout = t.Writer
		cmd.Stderr = t.Writer
		err := cmd.Run()
		if err != nil {
			slog.Error("task run error", slog.Any("err", err))
			return err
		}
		return nil
	}

	// grpc or grpcs
	if strings.Contains(t.Endpoint, "grpc://") ||
		strings.Contains(t.Endpoint, "grpcs://") {
		return fmt.Errorf("not support grpc")
		//exec grpc
		//conn, err := grpc.Dial(t.Endpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
		//if err != nil {
		//	slog.Error("task grpc dial error", slog.Any("err", err))
		//	return err
		//}
		//defer conn.Close()
		//stream, err := proto.NewTaskClient(conn).Exec(ctx, &proto.ExecRequest{
		//	Id:      t.ID,
		//	Name:    &t.Name,
		//	Command: t.Command,
		//	Args:    t.Args,
		//})
		//for {
		//	resp, err1 := stream.Recv()
		//	if err1 != nil {
		//		slog.Error("task grpc exec error", slog.Any("err", err))
		//		break
		//	}
		//	if resp == nil || len(resp.Content) == 0 {
		//		_, err1 = t.Writer.Write(resp.Content)
		//		if err1 != nil {
		//			slog.Error("task grpc write error", slog.Any("err", err))
		//			break
		//		}
		//	}
		//}
		//return nil
	}

	//http or https
	if strings.Contains(t.Endpoint, "http://") ||
		strings.Contains(t.Endpoint, "https://") {
		var body io.Reader
		switch t.Method {
		case http.MethodPut, http.MethodPost:
			data := map[string]any{
				"command": t.Command,
				"args":    t.Args,
			}
			b, _ := json.Marshal(data)
			body = bytes.NewReader(b)
		}
		req, err := http.NewRequest(t.Method, t.Endpoint, body)
		if err != nil {
			slog.Error("task http new request error", slog.Any("err", err))
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range t.Metadata {
			req.Header.Set(k, v)
		}
		client := &http.Client{
			Timeout: t.Timeout,
		}
		resp, err := client.Do(req)
		if err != nil {
			slog.Error("task http do error", slog.Any("err", err))
			return err
		}
		defer resp.Body.Close()
		_, err = io.Copy(t.Writer, resp.Body)
		if err != nil {
			slog.Error("task http copy error", slog.Any("err", err))
			return err
		}
		return nil
	}

	// ws or wss
	if strings.Contains(t.Endpoint, "ws://") ||
		strings.Contains(t.Endpoint, "wss://") {
		var protocol, origin string
		if t.Metadata != nil {
			protocol, _ = t.Metadata["protocol"]
			origin, _ = t.Metadata["origin"]
		}
		//new websocket
		conn, err := websocket.Dial(t.Endpoint, protocol, origin)
		if err != nil {
			slog.Error("task websocket dial error", slog.Any("err", err))
			return err
		}
		defer conn.Close()
		// Handle incoming messages
		go func(w io.Writer) {
			for {
				var message []byte
				err1 := websocket.Message.Receive(conn, &message)
				if err1 != nil {
					if errors.Is(err1, io.EOF) {
						break
					}
					slog.Error("task websocket receive error", slog.Any("err", err))
					break
				}
				_, err1 = w.Write(message)
				if err1 != nil {
					if errors.Is(err1, io.EOF) {
						break
					}
					slog.Error("task websocket write error", slog.Any("err", err))
					break
				}
			}
		}(t.Writer)

		<-time.NewTimer(t.Timeout).C
		return nil
	}
	slog.Error("not support", slog.String("endpoint", t.Endpoint))
	return fmt.Errorf("not support")
}
