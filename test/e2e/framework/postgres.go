package framework

import (
	"fmt"
	"time"

	"github.com/appscode/go/crypto/rand"
	jtypes "github.com/appscode/go/encoding/json/types"
	"github.com/appscode/go/types"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	kutildb "github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
	. "github.com/onsi/gomega"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (i *Invocation) Postgres() *api.Postgres {
	return &api.Postgres{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(api.ResourceSingularPostgres),
			Namespace: i.namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},
		Spec: api.PostgresSpec{
			Version:  jtypes.StrYo("10.2"),
			Replicas: types.Int32P(1),
		},
	}
}

func (f *Framework) CreatePostgres(obj *api.Postgres) error {
	_, err := f.extClient.Postgreses(obj.Namespace).Create(obj)
	return err
}

func (f *Framework) GetPostgres(meta metav1.ObjectMeta) (*api.Postgres, error) {
	return f.extClient.Postgreses(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
}

func (f *Framework) PatchPostgres(meta metav1.ObjectMeta, transform func(postgres *api.Postgres) *api.Postgres) (*api.Postgres, error) {
	postgres, err := f.extClient.Postgreses(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	postgres, _, err = kutildb.PatchPostgres(f.extClient, postgres, transform)
	return postgres, err
}

func (f *Framework) DeletePostgres(meta metav1.ObjectMeta) error {
	return f.extClient.Postgreses(meta.Namespace).Delete(meta.Name, &metav1.DeleteOptions{})
}

func (f *Framework) EventuallyPostgres(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := f.extClient.Postgreses(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
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

func (f *Framework) EventuallyPostgresPodCount(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() int32 {
			st, err := f.kubeClient.AppsV1beta1().StatefulSets(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			if err != nil {
				if kerr.IsNotFound(err) {
					return -1
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			}
			return st.Status.ReadyReplicas
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyPostgresRunning(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			postgres, err := f.extClient.Postgreses(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return postgres.Status.Phase == api.DatabasePhaseRunning
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) CleanPostgres() {
	postgresList, err := f.extClient.Postgreses(f.namespace).List(metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, e := range postgresList.Items {
		if _, _, err := kutildb.PatchPostgres(f.extClient, &e, func(in *api.Postgres) *api.Postgres {
			in.ObjectMeta.Finalizers = nil
			return in
		}); err != nil {
			fmt.Printf("error Patching Postgres. error: %v", err)
		}
	}
	if err := f.extClient.Postgreses(f.namespace).DeleteCollection(deleteInBackground(), metav1.ListOptions{}); err != nil {
		fmt.Printf("error in deletion of Postgres. Error: %v", err)
	}
}
