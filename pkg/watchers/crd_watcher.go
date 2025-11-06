package watchers

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

// CRDInfo represents information about a CRD
type CRDInfo struct {
	Name    string
	Group   string
	Version string
	Kind    string
}

// CRDEventHandler is called when a CRD becomes available or unavailable
type CRDEventHandler func(crd *CRDInfo)

// CRDWatcher watches for CRD availability and triggers handler registration
type CRDWatcher struct {
	config        *rest.Config
	dynamicClient dynamic.Interface
	factory       dynamicinformer.DynamicSharedInformerFactory
	logger        *zap.Logger

	// Map of GVK -> list of handlers to call when CRD appears
	watchedCRDs map[string][]CRDEventHandler
	mu          sync.RWMutex

	// Track which CRDs are currently available
	availableCRDs map[string]*CRDInfo
	availableMu   sync.RWMutex

	started bool
}

// NewCRDWatcher creates a new CRD watcher
func NewCRDWatcher(config *rest.Config, dynamicClient dynamic.Interface, factory dynamicinformer.DynamicSharedInformerFactory, logger *zap.Logger) *CRDWatcher {
	return &CRDWatcher{
		config:        config,
		dynamicClient: dynamicClient,
		factory:       factory,
		logger:        logger,
		watchedCRDs:   make(map[string][]CRDEventHandler),
		availableCRDs: make(map[string]*CRDInfo),
		started:       false,
	}
}

// WatchCRD registers interest in a CRD and calls handler when it becomes available
func (w *CRDWatcher) WatchCRD(group, kind string, onAvailable, onUnavailable CRDEventHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()

	key := fmt.Sprintf("%s.%s", group, kind)

	w.logger.Debug("registering CRD watch",
		zap.String("group", group),
		zap.String("kind", kind),
	)

	// Store the handlers
	if w.watchedCRDs[key] == nil {
		w.watchedCRDs[key] = make([]CRDEventHandler, 0)
	}

	// For now, we'll call onAvailable when CRD appears
	// Store both handlers for future use (you could extend this)
	if onAvailable != nil {
		w.watchedCRDs[key] = append(w.watchedCRDs[key], onAvailable)
	}

	// If already started and CRD is available, call handler immediately
	if w.started {
		w.availableMu.RLock()
		if crd, exists := w.availableCRDs[key]; exists {
			w.availableMu.RUnlock()
			w.logger.Info("CRD already available, triggering handler",
				zap.String("group", group),
				zap.String("kind", kind),
			)
			if onAvailable != nil {
				onAvailable(crd)
			}
		} else {
			w.availableMu.RUnlock()
		}
	}
}

// Start begins watching for CRDs
func (w *CRDWatcher) Start(ctx context.Context) error {
	w.logger.Info("starting CRD watcher")

	// Get the GVR for CRDs
	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}

	informer := w.factory.ForResource(crdGVR).Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			w.handleCRDAdd(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			w.handleCRDUpdate(oldObj, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			w.handleCRDDelete(obj)
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add CRD event handler: %w", err)
	}

	w.started = true
	w.logger.Info("CRD watcher started")

	return nil
}

// handleCRDAdd processes a newly added CRD
func (w *CRDWatcher) handleCRDAdd(obj interface{}) {
	crd, err := ConvertToTyped[apiextensionsv1.CustomResourceDefinition](obj)
	if err != nil {
		w.logger.Error("failed to convert to CRD", zap.Error(err))
		return
	}

	// Check if CRD is established (ready to use)
	established := false
	for _, condition := range crd.Status.Conditions {
		if condition.Type == apiextensionsv1.Established && condition.Status == apiextensionsv1.ConditionTrue {
			established = true
			break
		}
	}

	if !established {
		w.logger.Debug("CRD not yet established, skipping", zap.String("name", crd.Name))
		return
	}

	// Extract CRD information
	group := crd.Spec.Group
	kind := crd.Spec.Names.Kind

	// Get the storage version (preferred version)
	var version string
	for _, v := range crd.Spec.Versions {
		if v.Storage {
			version = v.Name
			break
		}
	}
	if version == "" && len(crd.Spec.Versions) > 0 {
		version = crd.Spec.Versions[0].Name
	}

	key := fmt.Sprintf("%s.%s", group, kind)

	crdInfo := &CRDInfo{
		Name:    crd.Name,
		Group:   group,
		Version: version,
		Kind:    kind,
	}

	// Mark as available
	w.availableMu.Lock()
	w.availableCRDs[key] = crdInfo
	w.availableMu.Unlock()

	w.logger.Debug("CRD became available",
		zap.String("name", crd.Name),
		zap.String("group", group),
		zap.String("kind", kind),
		zap.String("version", version),
	)

	// Trigger handlers
	w.mu.RLock()
	handlers := w.watchedCRDs[key]
	w.mu.RUnlock()

	for _, handler := range handlers {
		if handler != nil {
			handler(crdInfo)
		}
	}
}

// handleCRDUpdate processes an updated CRD
func (w *CRDWatcher) handleCRDUpdate(oldObj, newObj interface{}) {
	// For now, treat updates as adds (re-evaluate establishment status)
	w.handleCRDAdd(newObj)
}

// handleCRDDelete processes a deleted CRD
func (w *CRDWatcher) handleCRDDelete(obj interface{}) {
	crd, err := ConvertToTyped[apiextensionsv1.CustomResourceDefinition](obj)
	if err != nil {
		w.logger.Error("failed to convert to CRD", zap.Error(err))
		return
	}

	group := crd.Spec.Group
	kind := crd.Spec.Names.Kind
	key := fmt.Sprintf("%s.%s", group, kind)

	w.availableMu.Lock()
	delete(w.availableCRDs, key)
	w.availableMu.Unlock()

	w.logger.Info("CRD became unavailable",
		zap.String("name", crd.Name),
		zap.String("group", group),
		zap.String("kind", kind),
	)

	// TODO: Trigger onUnavailable handlers to stop watchers
	// For now, we'll leave handlers running (they'll just get no events)
}

// IsCRDAvailable checks if a specific CRD is currently available
func (w *CRDWatcher) IsCRDAvailable(group, kind string) bool {
	key := fmt.Sprintf("%s.%s", group, kind)
	w.availableMu.RLock()
	defer w.availableMu.RUnlock()
	_, exists := w.availableCRDs[key]
	return exists
}

// GetAvailableCRDs returns a list of all currently available CRDs
func (w *CRDWatcher) GetAvailableCRDs() []*CRDInfo {
	w.availableMu.RLock()
	defer w.availableMu.RUnlock()

	crds := make([]*CRDInfo, 0, len(w.availableCRDs))
	for _, crd := range w.availableCRDs {
		crds = append(crds, crd)
	}
	return crds
}
