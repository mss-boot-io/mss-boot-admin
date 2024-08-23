package pkg

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/8/23 11:34:49
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/8/23 11:34:49
 */
import (
	"context"
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

	return &clientcmdapi.Config{
		APIVersion: "v1",
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: {
				Server:                   aws.ToString(cluster.Endpoint),
				CertificateAuthorityData: []byte(*cluster.CertificateAuthority.Data),
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
				},
			},
		},
	}, nil
}
