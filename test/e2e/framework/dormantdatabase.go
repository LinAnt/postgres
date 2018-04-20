package framework

import (
	"fmt"
	"time"

	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	kutildb "github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
	. "github.com/onsi/gomega"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (f *Framework) GetDormantDatabase(meta metav1.ObjectMeta) (*api.DormantDatabase, error) {
	return f.extClient.DormantDatabases(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
}

func (f *Framework) PatchDormantDatabase(meta metav1.ObjectMeta, transform func(*api.DormantDatabase) *api.DormantDatabase) (*api.DormantDatabase, error) {
	dormantDatabase, err := f.extClient.DormantDatabases(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	dormantDatabase, _, err = kutildb.PatchDormantDatabase(f.extClient, dormantDatabase, transform)
	return dormantDatabase, err
}
func (f *Framework) DeleteDormantDatabase(meta metav1.ObjectMeta) error {
	return f.extClient.DormantDatabases(meta.Namespace).Delete(meta.Name, &metav1.DeleteOptions{})
}

func (f *Framework) EventuallyDormantDatabase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := f.extClient.DormantDatabases(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			if err != nil {
				if kerr.IsNotFound(err) {
					return false
				}
				Expect(err).NotTo(HaveOccurred())
			}
			return true
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyDormantDatabaseStatus(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() api.DormantDatabasePhase {
			drmn, err := f.extClient.DormantDatabases(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			if err != nil {
				if !kerr.IsNotFound(err) {
					Expect(err).NotTo(HaveOccurred())
				}
				return api.DormantDatabasePhase("")
			}
			return drmn.Status.Phase
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyWipedOut(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() error {
			labelMap := map[string]string{
				api.LabelDatabaseName: meta.Name,
				api.LabelDatabaseKind: api.ResourceKindPostgres,
			}
			labelSelector := labels.SelectorFromSet(labelMap)

			// check if pvcs is wiped out
			pvcList, err := f.kubeClient.CoreV1().PersistentVolumeClaims(meta.Namespace).List(
				metav1.ListOptions{
					LabelSelector: labelSelector.String(),
				},
			)
			if err != nil {
				return err
			}
			if len(pvcList.Items) > 0 {
				return fmt.Errorf("PVCs have not wiped out yet")
			}

			// check if snapshot is wiped out
			snapshotList, err := f.extClient.Snapshots(meta.Namespace).List(
				metav1.ListOptions{
					LabelSelector: labelSelector.String(),
				},
			)
			if err != nil {
				return err
			}
			if len(snapshotList.Items) > 0 {
				return fmt.Errorf("all snapshots have not wiped out yet")
			}

			// check if secrets are wiped out
			secretList, err := f.kubeClient.CoreV1().Secrets(meta.Namespace).List(
				metav1.ListOptions{
					LabelSelector: labelSelector.String(),
				},
			)
			if err != nil {
				return err
			}
			if len(secretList.Items) > 0 {
				return fmt.Errorf("secrets have not wiped out yet")
			}

			return nil
		},
		time.Minute*10,
		time.Second*5,
	)
}

func (f *Framework) CleanDormantDatabase() {
	dormantDatabaseList, err := f.extClient.DormantDatabases(f.namespace).List(metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, d := range dormantDatabaseList.Items {
		if _, _, err := kutildb.PatchDormantDatabase(f.extClient, &d, func(in *api.DormantDatabase) *api.DormantDatabase {
			in.ObjectMeta.Finalizers = nil
			return in
		}); err != nil {
			fmt.Printf("error Patching DormantDatabase. error: %v", err)
		}
	}
	if err := f.extClient.DormantDatabases(f.namespace).DeleteCollection(deleteInBackground(), metav1.ListOptions{}); err != nil {
		fmt.Printf("error in deletion of Dormant Database. Error: %v", err)
	}
}
