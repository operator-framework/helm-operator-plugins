package migrator

import (
	"fmt"
	"strings"

	v1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"k8s.io/client-go/rest"

	helm2to3 "github.com/helm/helm-2to3/pkg/v3"
	storagev3 "helm.sh/helm/v3/pkg/storage"
	driverv3 "helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	storagev2 "k8s.io/helm/pkg/storage"
	driverv2 "k8s.io/helm/pkg/storage/driver"
)

type MigratorGetter interface {
	MigratorFor(obj *unstructured.Unstructured) Migrator
}

func NewMigratorGetter(cfg *rest.Config) (MigratorGetter, error) {
	client, err := v1.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &migratorV2toV3Getter{client}, nil
}

type migratorV2toV3Getter struct {
	client v1.CoreV1Interface
}

func (m *migratorV2toV3Getter) MigratorFor(obj *unstructured.Unstructured) Migrator {
	driverV2 := driverv2.NewSecrets(m.client.Secrets(obj.GetNamespace()))
	driverV3 := driverv3.NewSecrets(m.client.Secrets(obj.GetNamespace()))
	return &migratorV2toV3{
		releaseName:      obj.GetName(),
		storageBackendV2: storagev2.Init(driverV2),
		storageBackendV3: storagev3.Init(driverV3),
	}
}

type Migrator interface {
	Migrate() error
}

type migratorV2toV3 struct {
	releaseName      string
	storageBackendV2 *storagev2.Storage
	storageBackendV3 *storagev3.Storage
}

func (m *migratorV2toV3) Migrate() error {
	historyV2, err := m.storageBackendV2.History(m.releaseName)
	if notFoundErr(err) || len(historyV2) == 0 {
		return nil
	}
	if err != nil {
		return err
	}

	for _, releaseV2 := range historyV2 {
		releaseV3, err := helm2to3.CreateRelease(releaseV2)
		if err != nil {
			return fmt.Errorf("generate v3 release: %w", err)
		}
		if err := m.storageBackendV3.Create(releaseV3); err != nil {
			return fmt.Errorf("generate v3 release: %w", err)
		}
		if _, err := m.storageBackendV2.Delete(releaseV2.GetName(), releaseV2.GetVersion()); err != nil {
			return fmt.Errorf("delete v2 release: %w", err)
		}
	}
	return nil
}

func notFoundErr(err error) bool {
	return strings.Contains(err.Error(), "not found")
}
