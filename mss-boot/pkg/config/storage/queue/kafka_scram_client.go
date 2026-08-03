package queue

import (
	"crypto/sha256"
	"crypto/sha512"

	"github.com/IBM/sarama"
	"github.com/xdg-go/scram"
)

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/4/30 19:34:34
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/4/30 19:34:34
 */

func SCRAMClientGeneratorFuncSHA256() sarama.SCRAMClient {
	return &XDGSCRAMClient{HashGeneratorFcn: sha256.New}
}

func SCRAMClientGeneratorFuncSHA512() sarama.SCRAMClient {
	return &XDGSCRAMClient{HashGeneratorFcn: sha512.New}
}

type XDGSCRAMClient struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

func (x *XDGSCRAMClient) Begin(userName, password, authzID string) (err error) {
	x.Client, err = x.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	x.ClientConversation = x.NewConversation()
	return nil
}

func (x *XDGSCRAMClient) Step(challenge string) (response string, err error) {
	response, err = x.ClientConversation.Step(challenge)
	return
}

func (x *XDGSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}
