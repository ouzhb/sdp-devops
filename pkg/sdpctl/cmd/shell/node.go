package shell

import (
	"bytes"
	"fmt"
	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"os"
	"sdp-devops/pkg/sdpctl/config"
	"sdp-devops/pkg/sdpctl/sdpk8s"
	k8stools "sdp-devops/pkg/util/kubernetes"
	systools "sdp-devops/pkg/util/sys"
	"strings"
)

func nodeShell(cmd *cobra.Command, args []string) {

	kubeClientSet, kubeClientConfig := k8stools.KubeClientAndConfig(config.KubeConfigStr)

	cmdStr := strings.Join(args, " ")
	// 返回所有需要运行运行的Node列表
	shellPodTargets := sdpk8s.GetShellPodDict(kubeClientSet, targetNode, targetNodeFile, toolName)
	i := 0
	threadNum := 0
	total := len(shellPodTargets)
	tChan := make(chan int, len(shellPodTargets))
	outPuts := make([]OutPut, len(shellPodTargets))
	defer close(tChan)

	for n, pod := range shellPodTargets {
		outPut := OutPut{
			Title:  fmt.Sprintf("------------------------------> No.%d  Shell on node: %s <------------------------------", i, n),
			StdOut: bytes.NewBufferString(""),
			StdErr: bytes.NewBufferString(""),
		}

		outPuts[i] = outPut
		if pod != nil {
			shExecOps := k8stools.ExecOptions{
				Command:       cmdStr,
				ContainerName: "",
				In:            nil,
				Out:           outPut.StdOut,
				Err:           outPut.StdErr,
				Istty:         false,
				TimeOut:       httpTimeOutInSec,
			}
			go execCmdParallel(kubeClientSet, kubeClientConfig, pod, shExecOps, tChan)
			threadNum += 1
		} else {
			outPut.StdErr.WriteString("Can't find shell pod on " + n)
		}
		i += 1
		if threadNum == currentThreadNum || total == i {
			systools.WaitAllThreadFinish(threadNum, tChan, httpTimeOutInSec)
			threadNum = 0
		}
	}
	for _, output := range outPuts {
		os.Stderr.WriteString(color.HiYellowString(output.Title) + "\n")
		color.HiGreen(output.StdOut.String())
		color.HiYellow(output.StdErr.String())
	}
}

func execCmdParallel(kubeClientSet *kubernetes.Clientset, kubeClientConfig *restclient.Config, pod *v1.Pod, execOptions k8stools.ExecOptions, tChan chan int) {
	err := k8stools.ExecCmd(kubeClientSet, kubeClientConfig, pod, execOptions)
	if err != nil {
		logrus.Println("请求 API Service 返回异常：", pod.Status.HostIP)
		//panic(err.Error())
	}
	tChan <- 1
}
