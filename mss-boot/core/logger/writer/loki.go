package writer

// import (
//	"bytes"
//	"errors"
//	"io"
//	"log/slog"
//	"net/http"
//	"os"
//	"os/signal"
//	"syscall"
//	"time"
//
//	"github.com/gogo/protobuf/proto"
//	"github.com/golang/snappy"
//	"github.com/grafana/loki/v3/pkg/logproto"
//	"github.com/prometheus/common/model"
//)
//
///*
// * @Author: lwnmengjing<lwnmengjing@qq.com>
// * @Date: 2024/5/7 18:37:00
// * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
// * @Last Modified time: 2024/5/7 18:37:00
// */
//
// type LokiWriter struct {
//	opts    Options
//	entries chan logproto.Entry
//}
//
//func NewLokiWriter(opts ...Option) (*LokiWriter, error) {
//	options := setDefault()
//	for _, o := range opts {
//		o(&options)
//	}
//	p := &LokiWriter{
//		opts:    options,
//		entries: make(chan logproto.Entry, options.bufferSize),
//	}
//	go p.write()
//	return p, nil
//}
//
//func (p *LokiWriter) Write(data []byte) (n int, err error) {
//	if p.entries == nil {
//		p.entries = make(chan logproto.Entry, p.opts.bufferSize)
//	}
//	n = len(data)
//	go func() {
//		p.entries <- logproto.Entry{
//			Line:      string(data),
//			Timestamp: time.Now(),
//		}
//	}()
//	return n, nil
//}
//
//func (p *LokiWriter) write() {
//	entries := make([]logproto.Entry, 0)
//
//	done := make(chan os.Signal, 1)
//	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
//	for {
//		select {
//		case <-done:
//			err := p.send(entries)
//			if err != nil {
//				slog.Error("application exit, send to loki failed", slog.String("error", err.Error()))
//				err = nil
//			}
//			return
//		case <-time.After(p.opts.lokiInterval):
//			// send to loki
//			if len(entries) > 0 {
//				err := p.send(entries)
//				if err != nil {
//					slog.Error("send to loki failed", slog.String("error", err.Error()))
//					err = nil
//				}
//				entries = make([]logproto.Entry, 0)
//			}
//		case d := <-p.entries:
//			entries = append(entries, d)
//		}
//	}
//}
//
//func (p *LokiWriter) send(entries []logproto.Entry) error {
//	if len(entries) == 0 {
//		return nil
//	}
//	// send to loki
//	labels := make(model.LabelSet)
//	for k, v := range p.opts.lokiLabels {
//		labels[model.LabelName(k)] = model.LabelValue(v)
//	}
//	req := &logproto.PushRequest{
//		Streams: []logproto.Stream{
//			{
//				Labels:  labels.String(),
//				Entries: entries,
//			},
//		},
//	}
//	payload, err := proto.Marshal(req)
//	if err != nil {
//		return err
//	}
//	payload = snappy.Encode(nil, payload)
//	// 发送POST请求到Loki
//	resp, err := http.Post(p.opts.lokiURL,
//		"application/x-protobuf",
//		bytes.NewBuffer(payload))
//	if err != nil {
//		return err
//	}
//	defer resp.Body.Close()
//	if resp.StatusCode > 399 {
//		var body []byte
//		body, err = io.ReadAll(resp.Body)
//		if err != nil {
//			return err
//		}
//		return errors.New(string(body))
//	}
//	return nil
//}
