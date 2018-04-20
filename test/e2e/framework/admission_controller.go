package framework

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/appscode/go/log"
	"github.com/kubedb/postgres/pkg/cmds/server"
	. "github.com/onsi/gomega"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kApi "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
)

var (
	DockerRegistry     string
	ExporterTag        string
	EnableRbac         bool
	ProvidedController bool
)

func (f *Framework) EventuallyAPIServiceReady() GomegaAsyncAssertion {
	return Eventually(
		func() error {
			crd, err := f.kaClient.ApiregistrationV1beta1().APIServices().Get("v1alpha1.admission.kubedb.com", metav1.GetOptions{})
			if err != nil {
				return err
			}
			for _, cond := range crd.Status.Conditions {
				if cond.Type == kApi.Available && cond.Status == kApi.ConditionTrue {
					log.Info("APIService status is true")
					time.Sleep(time.Second * 3) // let the resource become available
					return nil
				}
			}
			log.Error("APIService not ready yet")
			return fmt.Errorf("APIService not ready yet")
		},
		time.Minute*2,
		time.Second*5,
	)
}

func (f *Framework) RunOperatorAndServer(kubeconfigPath string, stopCh <-chan struct{}) {
	serverOpt := server.NewPostgresServerOptions(os.Stdout, os.Stderr)

	serverOpt.RecommendedOptions.CoreAPI.CoreAPIKubeconfigPath = kubeconfigPath
	serverOpt.RecommendedOptions.SecureServing.BindPort = 8443
	serverOpt.RecommendedOptions.SecureServing.BindAddress = net.ParseIP("127.0.0.1")
	serverOpt.RecommendedOptions.Authorization.RemoteKubeConfigFile = kubeconfigPath
	serverOpt.RecommendedOptions.Authentication.RemoteKubeConfigFile = kubeconfigPath

	serverOpt.ExtraOptions.Docker.Registry = DockerRegistry
	serverOpt.ExtraOptions.Docker.ExporterTag = ExporterTag
	serverOpt.ExtraOptions.EnableRBAC = EnableRbac

	err := serverOpt.Run(stopCh)
	Expect(err).NotTo(HaveOccurred())
}

func (f *Framework) CleanAdmissionConfigs() {
	// delete validating Webhook
	if err := f.kubeClient.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().DeleteCollection(deleteInBackground(), metav1.ListOptions{
		LabelSelector: "app=kubedb",
	}); err != nil && !kerr.IsNotFound(err) {
		fmt.Printf("error in deletion of Validating Webhook. Error: %v", err)
	}

	// delete mutating Webhook
	if err := f.kubeClient.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().DeleteCollection(deleteInBackground(), metav1.ListOptions{
		LabelSelector: "app=kubedb",
	}); err != nil && !kerr.IsNotFound(err) {
		fmt.Printf("error in deletion of Mutating Webhook. Error: %v", err)
	}

	// Delete APIService
	if err := f.kaClient.ApiregistrationV1beta1().APIServices().DeleteCollection(deleteInBackground(), metav1.ListOptions{
		LabelSelector: "app=kubedb",
	}); err != nil && !kerr.IsNotFound(err) {
		fmt.Printf("error in deletion of APIService. Error: %v", err)
	}

	// Delete Service
	if err := f.kubeClient.CoreV1().Services("kube-system").Delete("kubedb-operator", &metav1.DeleteOptions{}); err != nil && !kerr.IsNotFound(err) {
		fmt.Printf("error in deletion of Service. Error: %v", err)
	}

	// Delete EndPoints
	if err := f.kubeClient.CoreV1().Endpoints("kube-system").DeleteCollection(deleteInBackground(), metav1.ListOptions{
		LabelSelector: "app=kubedb",
	}); err != nil && !kerr.IsNotFound(err) {
		fmt.Printf("error in deletion of Endpoints. Error: %v", err)
	}

	time.Sleep(time.Second * 1) // let the kube-server know it!!
}
