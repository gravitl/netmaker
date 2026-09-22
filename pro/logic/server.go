package logic

var DeploymentMode string

func SetDeploymentMode(deploymentMode string) {
	DeploymentMode = deploymentMode
}

func GetDeploymentMode() string {
	return DeploymentMode
}
