package pkg

import (
	"context"
	"encoding/base64"
	"encoding/pem"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// FetchEKSKubeconfig fetches the kubeconfig for the specified EKS cluster.
func FetchEKSKubeconfig(ctx context.Context, svc *eks.Client, region, clusterName string) (*clientcmdapi.Config, error) {
	result, err := svc.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return nil, err
	}

	cluster := result.Cluster
	if cluster == nil {
		return nil, fmt.Errorf("cluster %s not found", clusterName)
	}

	// Decode and validate the Certificate Authority data
	certData, err := base64.StdEncoding.DecodeString(aws.ToString(cluster.CertificateAuthority.Data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode certificate authority data: %w", err)
	}

	pemBlock, _ := pem.Decode(certData)
	if pemBlock == nil {
		return nil, fmt.Errorf("failed to parse certificate authority data as PEM block")
	}

	return &clientcmdapi.Config{
		APIVersion: "v1",
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: {
				Server:                   aws.ToString(cluster.Endpoint),
				CertificateAuthorityData: certData,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"default": {
				Cluster:  clusterName,
				AuthInfo: clusterName,
			},
		},
		CurrentContext: "default",
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			clusterName: {
				Exec: &clientcmdapi.ExecConfig{
					APIVersion: "client.authentication.k8s.io/v1beta1",
					Command:    "aws",
					Args: []string{
						"--region",
						region,
						"eks",
						"get-token",
						"--cluster-name",
						clusterName,
					},
					InteractiveMode:    clientcmdapi.IfAvailableExecInteractiveMode,
					ProvideClusterInfo: false,
				},
			},
		},
	}, nil
}
