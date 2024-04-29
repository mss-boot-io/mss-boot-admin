package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	consul "github.com/hashicorp/consul/api"
)

func main() {
	// 创建Consul API客户端配置
	config := consul.DefaultConfig()
	config.Address = "localhost:8500" // Consul服务器的地址和端口

	// 创建Consul API客户端
	client, err := consul.NewClient(config)
	if err != nil {
		log.Fatalf("Error creating Consul client: %s", err)
	}

	rb, err := os.ReadFile("../../config/application.yml")
	if err != nil {
		log.Fatalf("Error reading file: %s", err)
	}
	key := "mss-boot-admin/config/application.yml"
	// 写入键值对配置
	pair := &consul.KVPair{
		Key:   key,
		Value: rb,
	}
	_, err = client.KV().Put(pair, nil)
	if err != nil {
		log.Fatalf("Error writing to Consul KV: %s", err)
	}
	fmt.Println("Successfully wrote to Consul KV")

	// 读取键值对配置
	readPair, _, err := client.KV().Get(key, nil)
	if err != nil {
		log.Fatalf("Error reading from Consul KV: %s", err)
	}

	// 如果找到键值对，则打印值
	if readPair != nil {
		fmt.Printf("Value for key %s: %s\n", key, string(readPair.Value))
	} else {
		fmt.Printf("Key %s not found in Consul\n", key)
	}

	params := &consul.QueryOptions{WaitIndex: 0, WaitTime: 10 * time.Second}
	for {
		// 获取最新的配置变化
		pairs, meta, err := client.KV().List(key, params)
		if err != nil {
			slog.Error("Error watching for config changes", slog.Any("err", err))
			continue
		}
		fmt.Println(meta.LastIndex)
		if pairs != nil {
			fmt.Printf("Config updated: %s\n", pair.Value)
		}
		params.WaitIndex = meta.LastIndex
	}
}
