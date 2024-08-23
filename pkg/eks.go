package pkg

/*
 * @Author: lwnmengjing<lwnmengjing@qq.com>
 * @Date: 2024/8/23 11:34:49
 * @Last Modified by: lwnmengjing<lwnmengjing@qq.com>
 * @Last Modified time: 2024/8/23 11:34:49
 */
import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"sigs.k8s.io/yaml"
)

// FetchEKSKubeconfig fetches the kubeconfig for the specified EKS cluster.
func FetchEKSKubeconfig(ctx context.Context, svc *eks.Client, clusterName string) (string, error) {
	result, err := svc.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(clusterName),
	})
	if err != nil {
		return "", err
	}

	cluster := result.Cluster
	if cluster == nil {
		return "", fmt.Errorf("cluster %s not found", clusterName)
	}

	kubeconfig := map[string]interface{}{
		"apiVersion": "v1",
		"clusters": []map[string]interface{}{
			{
				"cluster": map[string]interface{}{
					"server":                     aws.ToString(cluster.Endpoint),
					"certificate-authority-data": aws.ToString(cluster.CertificateAuthority.Data),
				},
				"name": clusterName,
			},
		},
		"contexts": []map[string]interface{}{
			{
				"context": map[string]interface{}{
					"cluster": clusterName,
					"user":    clusterName,
				},
				"name": "default",
			},
		},
		"current-context": "default",
		"users": []map[string]interface{}{
			{
				"name": clusterName,
				"user": map[string]interface{}{
					"exec": map[string]interface{}{
						"apiVersion": "client.authentication.k8s.io/v1beta1",
						"command":    "aws",
						"args": []string{
							"--region", "ap-northeast-1",
							"eks", "get-token",
							"--cluster-name", clusterName,
						},
						"interactiveMode":    "IfAvailable",
						"provideClusterInfo": false,
					},
				},
			},
		},
	}

	kubeconfigBytes, err := json.MarshalIndent(kubeconfig, "", "  ")
	if err != nil {
		return "", err
	}

	// Convert JSON to YAML
	kubeconfigYAML, err := yaml.JSONToYAML(kubeconfigBytes)
	if err != nil {
		return "", err
	}

	return string(kubeconfigYAML), nil
}
