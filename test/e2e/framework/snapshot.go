package framework

import (
	"fmt"
	"time"

	"github.com/appscode/go/crypto/rand"
	"github.com/graymeta/stow"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	kutildb "github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
	"github.com/kubedb/apimachinery/pkg/storage"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (i *Invocation) Snapshot() *api.Snapshot {
	return &api.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(api.ResourceSingularSnapshot),
			Namespace: i.namespace,
			Labels: map[string]string{
				"app": i.app,
				api.LabelDatabaseKind: api.ResourceKindPostgres,
			},
		},
	}
}

func (f *Framework) CreateSnapshot(obj *api.Snapshot) error {
	_, err := f.extClient.Snapshots(obj.Namespace).Create(obj)
	return err
}

func (f *Framework) GetSnapshot(meta metav1.ObjectMeta) (*api.Snapshot, error) {
	return f.extClient.Snapshots(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
}

func (f *Framework) DeleteSnapshot(meta metav1.ObjectMeta) error {
	return f.extClient.Snapshots(meta.Namespace).Delete(meta.Name, &metav1.DeleteOptions{})
}

func (f *Framework) EventuallySnapshotPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() api.SnapshotPhase {
			snapshot, err := f.extClient.Snapshots(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(snapshot.Status.Phase).ToNot(Equal(api.SnapshotPhaseFailed))
			return snapshot.Status.Phase
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallySnapshotDataFound(snapshot *api.Snapshot) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			found, err := f.checkSnapshotData(snapshot)
			Expect(err).NotTo(HaveOccurred())
			return found
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallySnapshotCount(meta metav1.ObjectMeta) GomegaAsyncAssertion {

	labelMap := map[string]string{
		api.LabelDatabaseKind: api.ResourceKindPostgres,
		api.LabelDatabaseName: meta.Name,
	}

	return Eventually(
		func() int {
			snapshotList, err := f.extClient.Snapshots(meta.Namespace).List(metav1.ListOptions{
				LabelSelector: labels.SelectorFromSet(labelMap).String(),
			})
			Expect(err).NotTo(HaveOccurred())

			return len(snapshotList.Items)
		},
		time.Minute*10,
		time.Second*5,
	)
}

func (f *Framework) checkSnapshotData(snapshot *api.Snapshot) (bool, error) {
	storageSpec := snapshot.Spec.SnapshotStorageSpec
	cfg, err := storage.NewOSMContext(f.kubeClient, storageSpec, snapshot.Namespace)
	if err != nil {
		return false, err
	}

	loc, err := stow.Dial(cfg.Provider, cfg.Config)
	if err != nil {
		return false, err
	}
	containerID, err := storageSpec.Container()
	if err != nil {
		return false, err
	}
	container, err := loc.Container(containerID)
	if err != nil {
		return false, err
	}

	folderName, _ := snapshot.Location()
	prefix := fmt.Sprintf("%v/%v", folderName, snapshot.Name)
	cursor := stow.CursorStart
	totalItem := 0
	for {
		items, next, err := container.Items(prefix, cursor, 50)
		if err != nil {
			return false, err
		}

		totalItem = totalItem + len(items)

		cursor = next
		if stow.IsCursorEnd(cursor) {
			break
		}
	}

	return totalItem != 0, nil
}

func (f *Framework) CleanSnapshot() {
	snapshotList, err := f.extClient.Snapshots(f.namespace).List(metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, s := range snapshotList.Items {
		if _, _, err := kutildb.PatchSnapshot(f.extClient, &s, func(in *api.Snapshot) *api.Snapshot {
			in.ObjectMeta.Finalizers = nil
			return in
		}); err != nil {
			fmt.Printf("error Patching Snapshot. error: %v", err)
		}
	}
	if err := f.extClient.Snapshots(f.namespace).DeleteCollection(deleteInBackground(), metav1.ListOptions{}); err != nil {
		fmt.Printf("error in deletion of Snapshot. Error: %v", err)
	}
}
