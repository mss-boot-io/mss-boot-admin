/*
 * @Author: lwnmengjing
 * @Date: 2021/5/30 7:30 下午
 * @Last Modified by: lwnmengjing
 * @Last Modified time: 2021/5/30 7:30 下午
 */

package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nsqio/go-nsq"

	"github.com/mss-boot-io/mss-boot/pkg/config/storage"
)

// NewNSQ nsq模式 只能监听一个channel
func NewNSQ(cfg *nsq.Config, lookup, adminAddr string, addresses ...string) (*NSQ, error) {
	n := &NSQ{
		lookupAddr: lookup,
		addresses:  addresses,
		adminAddr:  adminAddr,
		cfg:        cfg,
	}
	// 通过adminaddr获取节点信息
	err := n.queryNSQAdmin()
	if err != nil {
		return nil, err
	}
	return n, nil
}

type NSQ struct {
	addresses  []string
	lookupAddr string
	adminAddr  string
	cfg        *nsq.Config
	producer   []*nsq.Producer
	consumer   map[string]struct {
		*nsq.Consumer
		partition int
	}
	mux sync.Mutex
}

// String 字符串类型
func (*NSQ) String() string {
	return "nsq"
}

func (e *NSQ) newProducers() error {
	e.mux.Lock()
	defer e.mux.Unlock()
	if e.cfg == nil {
		e.cfg = nsq.NewConfig()
	}
	var err error
	e.producer = make([]*nsq.Producer, len(e.addresses))
	for i := range e.addresses {
		e.producer[i], err = nsq.NewProducer(e.addresses[i], e.cfg)
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *NSQ) getProducer(id string) *nsq.Producer {
	// 获取字符串hashcode
	hash := int(crc32.ChecksumIEEE([]byte(id)))
	// 取余
	index := hash % len(e.producer)
	return e.producer[index]
}

func (e *NSQ) newConsumer(topic, channel string, partition int, h nsq.Handler) (err error) {
	if e.cfg == nil {
		e.cfg = nsq.NewConfig()
	}
	if e.consumer == nil {
		e.consumer = make(map[string]struct {
			*nsq.Consumer
			partition int
		})
	}

	consumer, ok := e.consumer[fmt.Sprintf("%s:%s", topic, channel)]
	if !ok {
		consumer = struct {
			*nsq.Consumer
			partition int
		}{
			partition: partition,
		}
		consumer.Consumer, err = nsq.NewConsumer(topic, channel, e.cfg)
		if err != nil {
			return err
		}
		e.consumer[fmt.Sprintf("%s:%s", topic, channel)] = consumer
	}
	consumer.AddHandler(h)
	e.consumer[fmt.Sprintf("%s:%s", topic, channel)] = consumer
	return err
}

// Append 消息入生产者
func (e *NSQ) Append(opts ...storage.Option) error {
	if e.producer == nil {
		err := e.newProducers()
		if err != nil {
			return err
		}
	}
	o := storage.SetOptions(opts...)
	rb, err := json.Marshal(o.Message.GetValues())
	if err != nil {
		return err
	}
	if o.Delay > 0 {
		// 延时消息
		return e.getProducer(o.Message.GetID()).DeferredPublish(o.Message.GetStream(), o.Delay, rb)
	}
	return e.getProducer(o.Message.GetID()).Publish(o.Message.GetStream(), rb)
}

// Register 监听消费者
func (e *NSQ) Register(opts ...storage.Option) {
	o := storage.SetOptions(opts...)
	h := &nsqConsumerHandler{o.F}
	err := e.newConsumer(o.Topic, o.GroupID, o.Partition, h)
	if err != nil {
		slog.Error("nsq consumer register error", slog.Any("err", err))
		os.Exit(-1)
	}
}

func (e *NSQ) ping() {
	for {
		for i := range e.producer {
			err := e.producer[i].Ping()
			if err != nil {
				slog.Error("nsq producer ping error", slog.Any("err", err))
			}
		}
		time.Sleep(5 * time.Second)
	}
}

func (e *NSQ) Run(context.Context) {
	for i := range e.consumer {
		if e.lookupAddr != "" && e.consumer[i].partition < 0 {
			err := e.consumer[i].ConnectToNSQLookupd(e.lookupAddr)
			if err != nil {
				slog.Error("nsq consumer connect to nsqlookupd error", slog.Any("err", err))
				os.Exit(-1)
			}
			continue
		}
		if e.consumer[i].partition > -1 {
			partition := e.consumer[i].partition % len(e.addresses)
			err := e.consumer[i].ConnectToNSQDs([]string{e.addresses[partition]})
			if err != nil {
				slog.Error("select consumer by partition failed", "err", err)
				os.Exit(-1)
			}
		}
		err := e.consumer[i].ConnectToNSQDs(e.addresses)
		if err != nil {
			slog.Error("select consumer by address failed", "err", err)
			os.Exit(-1)
		}
	}
	e.ping()
}

func (e *NSQ) Shutdown() {
	for i := range e.producer {
		e.producer[i].Stop()
	}
	if e.consumer != nil {
		for k := range e.consumer {
			e.consumer[k].Stop()
		}
	}
}

func (e *NSQ) queryNSQAdmin() error {
	if e.adminAddr == "" {
		return nil
	}
	endpoint := e.adminAddr
	if !strings.Contains(endpoint, "http") {
		endpoint = fmt.Sprintf("http://%s", endpoint)
	}

	var data NodesResp
	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/nodes", endpoint), http.NoBody)
	if err != nil {
		slog.Error("error creating HTTP request to nsq admin", slog.Any("err", err))
		return err
	}
	if e.cfg.AuthSecret != "" && e.cfg.LookupdAuthorization {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.cfg.AuthSecret))
	}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("error querying nsq admin", slog.Any("err", err))
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		slog.Error("error querying nsq admin", slog.Any("status_code", resp.StatusCode))
		return err
	}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		slog.Error("error decoding nsq admin response", slog.Any("err", err))
		return err
	}

	for i := range data.Nodes {
		broadcastAddress := data.Nodes[i].BroadcastAddress
		port := data.Nodes[i].TCPPort
		joined := net.JoinHostPort(broadcastAddress, strconv.Itoa(port))
		var exist bool
		for j := range e.addresses {
			if e.addresses[j] == joined {
				exist = true
				break
			}
		}
		if !exist {
			e.addresses = append(e.addresses, joined)
		}
	}
	return nil
}

type NodesResp struct {
	Nodes   []*peerInfo `json:"nodes"`
	Message string      `json:"message"`
}

type peerInfo struct {
	RemoteAddress    string `json:"remote_address"`
	Hostname         string `json:"hostname"`
	BroadcastAddress string `json:"broadcast_address"`
	TCPPort          int    `json:"tcp_port"`
	HTTPPort         int    `json:"http_port"`
	Version          string `json:"version"`
}

type nsqConsumerHandler struct {
	f storage.ConsumerFunc
}

func (e nsqConsumerHandler) HandleMessage(message *nsq.Message) error {
	m := new(Message)
	data := make(map[string]any)
	err := json.Unmarshal(message.Body, &data)
	if err != nil {
		return err
	}
	m.SetValues(data)
	return e.f(m)
}
