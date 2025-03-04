package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	avastretry "github.com/avast/retry-go"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/chart"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/clientset"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/cni"
	ingressgateway "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/ingress-gateway"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/istioctl"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/manifest"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/merge"
	istioConfig "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/config"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/data"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/proxy"
	"github.com/kyma-incubator/reconciler/pkg/reconciler/kubernetes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	helmChart "helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

const (
	retriesCount        = 5
	delayBetweenRetries = 5 * time.Second
	timeout             = 5 * time.Minute
	interval            = 12 * time.Second
)

type VersionType string

type IstioStatus struct {
	ClientVersion     string
	TargetVersion     string
	TargetPrefix      string
	PilotVersion      string
	DataPlaneVersions map[string]bool
}

type IstioVersionOutput struct {
	ClientVersion    *ClientVersion      `json:"clientVersion"`
	MeshVersion      []*MeshComponent    `json:"meshVersion,omitempty"`
	DataPlaneVersion []*DataPlaneVersion `json:"dataPlaneVersion,omitempty"`
}

type ClientVersion struct {
	Version string `json:"version"`
}

type MeshComponent struct {
	Component string    `json:"Component,omitempty"`
	Info      *MeshInfo `json:"Info,omitempty"`
}

type MeshInfo struct {
	Version string `json:"version,omitempty"`
}

type DataPlaneVersion struct {
	IstioVersion string `json:"IstioVersion,omitempty"`
}

type chartValues struct {
	Global struct {
		SidecarMigration bool `json:"sidecarMigration"`
		Images           struct {
			IstioPilot struct {
				Version string `json:"version"`
			} `json:"istio_pilot"`
			IstioProxyV2 struct {
				Directory             string `json:"directory"`
				ContainerRegistryPath string `json:"containerRegistryPath"`
			} `json:"istio_proxyv2"`
		} `json:"images"`
	} `json:"global"`
	HelmValues struct {
		SidecarInjectorWebhook struct {
			EnableNamespacesByDefault bool `json:"enableNamespacesByDefault"`
		} `json:"sidecarInjectorWebhook"`
	} `json:"helmValues"`
}

// IstioPerformer performs actions on Istio component on the cluster.
//
//go:generate mockery --name=IstioPerformer --outpkg=mock --case=underscore
type IstioPerformer interface {

	// Install Istio in given version on the cluster using istioChart.
	Install(context context.Context, kubeConfig, istioChart, version string, logger *zap.SugaredLogger) error

	// LabelNamespaces labels all namespaces with enabled istio sidecar migration.
	LabelNamespaces(context context.Context, kubeClient kubernetes.Client, workspace chart.Factory, branchVersion string, istioChart string, logger *zap.SugaredLogger) error

	// Update Istio on the cluster to the targetVersion using istioChart.
	Update(context context.Context, kubeConfig, istioChart, targetVersion string, logger *zap.SugaredLogger) error

	// ResetProxy resets Istio proxy of all Istio sidecars on the cluster. The proxyImageVersion parameter controls the Istio proxy version.
	ResetProxy(context context.Context, kubeConfig string, workspace chart.Factory, branchVersion string, istioChart string, proxyImageVersion string, proxyImagePrefix string, logger *zap.SugaredLogger) error

	// Version reports status of Istio installation on the cluster.
	Version(workspace chart.Factory, branchVersion string, istioChart string, kubeConfig string, logger *zap.SugaredLogger) (IstioStatus, error)

	// Uninstall Istio from the cluster and its corresponding resources, using given Istio version.
	Uninstall(kubeClientSet kubernetes.Client, version string, logger *zap.SugaredLogger) error
}

// CommanderResolver interface implementations must be able to provide istioctl.Commander instances for given istioctl.Version
type CommanderResolver interface {
	// GetCommander function returns istioctl.Commander instance for given istioctl version if supported, returns an error otherwise.
	GetCommander(version istioctl.Version) (istioctl.Commander, error)
}

// DefaultIstioPerformer provides a default implementation of IstioPerformer.
// It uses istioctl binary to do its job. It delegates the job of finding proper istioctl binary for given operation to the configured CommandResolver.
type DefaultIstioPerformer struct {
	resolver        CommanderResolver
	istioProxyReset proxy.IstioProxyReset
	provider        clientset.Provider
	gatherer        data.Gatherer
}

// NewDefaultIstioPerformer creates a new instance of the DefaultIstioPerformer.
func NewDefaultIstioPerformer(resolver CommanderResolver, istioProxyReset proxy.IstioProxyReset, provider clientset.Provider, gatherer data.Gatherer) *DefaultIstioPerformer {
	return &DefaultIstioPerformer{resolver, istioProxyReset, provider, gatherer}
}

func (c *DefaultIstioPerformer) Uninstall(kubeClientSet kubernetes.Client, version string, logger *zap.SugaredLogger) error {
	logger.Debug("Starting Istio uninstallation...")

	execVersion, err := istioctl.VersionFromString(version)
	if err != nil {
		return errors.Wrap(err, "Error parsing version")
	}

	commander, err := c.resolver.GetCommander(execVersion)
	if err != nil {
		return err
	}

	err = commander.Uninstall(kubeClientSet.Kubeconfig(), logger)
	if err != nil {
		return errors.Wrap(err, "Error occurred when calling istioctl")
	}
	logger.Debug("Istio uninstall triggered")
	kubeClient, err := kubeClientSet.Clientset()
	if err != nil {
		return err
	}

	policy := metav1.DeletePropagationForeground
	err = kubeClient.CoreV1().Namespaces().Delete(context.TODO(), "istio-system", metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})
	if err != nil {
		return err
	}
	logger.Debug("Istio namespace deleted")
	return nil
}

func (c *DefaultIstioPerformer) Install(context context.Context, kubeConfig, istioChart, version string, logger *zap.SugaredLogger) error {
	logger.Debug("Starting Istio installation...")

	execVersion, err := istioctl.VersionFromString(version)
	if err != nil {
		return errors.Wrap(err, "Error parsing version")
	}

	istioOperatorManifest, err := manifest.ExtractIstioOperatorContextFrom(istioChart)
	if err != nil {
		return err
	}

	mergedIstioConfig, err := merge.IstioOperatorConfiguration(context, c.provider, istioOperatorManifest, kubeConfig, logger)
	if err != nil {
		return err
	}

	mergedCNI, err := cni.ApplyCNIConfiguration(context, c.provider, mergedIstioConfig, kubeConfig, logger)
	if err != nil {
		return err
	}

	commander, err := c.resolver.GetCommander(execVersion)
	if err != nil {
		return err
	}

	err = commander.Install(mergedCNI, kubeConfig, logger)
	if err != nil {
		return errors.Wrap(err, "Error occurred when calling istioctl")
	}

	installedVersion, err := getInstalledIstioVersion(c.provider, kubeConfig, c.gatherer, logger)
	if err != nil {
		return err
	}

	if execVersion.MajorMinorPatch() != installedVersion {
		return fmt.Errorf("Installed Istio version: %s do not match target version: %s", installedVersion, execVersion.MajorMinorPatch())
	}

	logger.Infof("Istio in version %s successfully installed", version)

	return nil
}

func (c *DefaultIstioPerformer) LabelNamespaces(context context.Context, kubeClient kubernetes.Client, workspace chart.Factory, branchVersion string, istioChart string, logger *zap.SugaredLogger) error {
	logger.Debugf("Labeling namespaces with istio-injection: enabled")
	clientSet, err := kubeClient.Clientset()
	if err != nil {
		return err
	}

	labelPatch := `{"metadata": {"labels": {"istio-injection": "enabled"}}}`

	sidecarMigrationEnabled, sidecarMigrationIsSet, err := isSidecarMigrationEnabled(workspace, branchVersion, istioChart)
	if err != nil {
		return err
	}
	if sidecarMigrationEnabled && sidecarMigrationIsSet {
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			namespaces, err := clientSet.CoreV1().Namespaces().List(context, metav1.ListOptions{})
			if err != nil {
				return err
			}
			for _, namespace := range namespaces.Items {
				_, isIstioInjectionSet := namespace.Labels["istio-injection"]
				if !isIstioInjectionSet && namespace.ObjectMeta.Name != "kube-system" {
					logger.Debugf("Patching namespace %s with label istio-injection: enabled", namespace.ObjectMeta.Name)
					_, err = clientSet.CoreV1().Namespaces().Patch(context, namespace.ObjectMeta.Name, types.MergePatchType, []byte(labelPatch), metav1.PatchOptions{})
				}
			}
			return err
		})
		if err != nil {
			return err
		}

		logger.Debugf("Namespaces have been labeled successfully")
	} else {
		logger.Debugf("Sidecar migration is disabled or it is not set, skipping labeling namespaces")
	}

	return nil
}

func (c *DefaultIstioPerformer) Update(context context.Context, kubeConfig, istioChart, targetVersion string, logger *zap.SugaredLogger) error {
	logger.Debug("Starting Istio update...")

	version, err := istioctl.VersionFromString(targetVersion)
	if err != nil {
		return errors.Wrap(err, "Error parsing version")
	}

	istioOperatorManifest, err := manifest.ExtractIstioOperatorContextFrom(istioChart)
	if err != nil {
		return err
	}

	ingressGatewayNeedsRestart, err := merge.NeedsIngressGatewayRestart(context, c.provider, kubeConfig, logger)
	if err != nil {
		return err
	}

	mergedIstioConfig, err := merge.IstioOperatorConfiguration(context, c.provider, istioOperatorManifest, kubeConfig, logger)
	if err != nil {
		return err
	}
	mergedCNI, err := cni.ApplyCNIConfiguration(context, c.provider, mergedIstioConfig, kubeConfig, logger)
	if err != nil {
		return err
	}

	commander, err := c.resolver.GetCommander(version)
	if err != nil {
		return err
	}

	err = commander.Upgrade(mergedCNI, kubeConfig, logger)
	if err != nil {
		return errors.Wrap(err, "Error occurred when calling istioctl")
	}

	updatedVersion, err := getInstalledIstioVersion(c.provider, kubeConfig, c.gatherer, logger)
	if err != nil {
		return err
	}

	if version.MajorMinorPatch() != updatedVersion {
		return fmt.Errorf("Updated Istio version: %s do not match target version: %s", updatedVersion, version.MajorMinorPatch())
	}

	logger.Infof("Istio has been updated successfully to version %s", targetVersion)

	if ingressGatewayNeedsRestart {
		logger.Infof("Restarting ingress-gateway")
		istioClient, err := c.provider.GetIstioClient(kubeConfig)
		if err != nil {
			return err
		}
		err = ingressgateway.RestartDeployment(context, istioClient)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *DefaultIstioPerformer) ResetProxy(context context.Context, kubeConfig string, workspace chart.Factory, branchVersion string, istioChart string, proxyImageVersion string, proxyImagePrefix string, logger *zap.SugaredLogger) error {
	kubeClient, err := c.provider.RetrieveFrom(kubeConfig, logger)
	if err != nil {
		logger.Error("Could not retrieve KubeClient from Kubeconfig!")
		return err
	}
	dynamicClient, err := c.provider.GetDynamicClient(kubeConfig)
	if err != nil {
		logger.Error("Could not retrieve Dynamic client from Kubeconfig!")
		return err
	}
	cniEnabled, err := cni.GetActualCNIState(dynamicClient)
	if err != nil {
		return err
	}

	sidecarInjectionEnabledByDefault, err := IsSidecarInjectionNamespacesByDefaultEnabled(workspace, branchVersion, istioChart)
	if err != nil {
		logger.Error("Could not retrieve default istio sidecar injection!")
		return err
	}

	cfg := istioConfig.IstioProxyConfig{
		IsUpdate:                         true,
		Context:                          context,
		ImagePrefix:                      proxyImagePrefix,
		ImageVersion:                     proxyImageVersion,
		RetriesCount:                     retriesCount,
		DelayBetweenRetries:              delayBetweenRetries,
		Timeout:                          timeout,
		Interval:                         interval,
		Kubeclient:                       kubeClient,
		Debug:                            false,
		Log:                              logger,
		SidecarInjectionByDefaultEnabled: sidecarInjectionEnabledByDefault,
		CNIEnabled:                       cniEnabled,
	}

	err = c.istioProxyReset.Run(cfg)
	if err != nil {
		return errors.Wrap(err, "Istio proxy reset error")
	}

	return nil
}

func (c *DefaultIstioPerformer) Version(workspace chart.Factory, branchVersion string, istioChart string, kubeConfig string, logger *zap.SugaredLogger) (IstioStatus, error) {
	targetVersion, err := getTargetVersionFromIstioChart(workspace, branchVersion, istioChart, logger)
	if err != nil {
		return IstioStatus{}, errors.Wrap(err, "Target Version could not be found")
	}

	targetPrefix, err := getTargetProxyV2PrefixFromIstioChart(workspace, branchVersion, istioChart, logger)
	if err != nil {
		return IstioStatus{}, errors.Wrap(err, "Target Prefix could not be found")
	}

	version, err := istioctl.VersionFromString(targetVersion)
	if err != nil {
		return IstioStatus{}, errors.Wrap(err, "Error parsing version")
	}

	commander, err := c.resolver.GetCommander(version)
	if err != nil {
		return IstioStatus{}, err
	}

	versionOutput, err := commander.Version(kubeConfig, logger)
	if err != nil {
		return IstioStatus{}, err
	}

	mappedIstioVersion, err := mapVersionToStruct(versionOutput, targetVersion, targetPrefix)

	return mappedIstioVersion, err
}

func getTargetVersionFromIstioChart(workspace chart.Factory, branch string, istioChart string, logger *zap.SugaredLogger) (string, error) {
	ws, err := workspace.Get(branch)
	if err != nil {
		return "", err
	}

	istioHelmChart, err := loader.Load(filepath.Join(ws.ResourceDir, istioChart))
	if err != nil {
		return "", err
	}

	pilotVersion, err := getTargetVersionFromPilotInChartValues(istioHelmChart)
	if err != nil {
		return "", err
	}

	if pilotVersion != "" {
		logger.Debugf("Resolved target Istio version: %s from values", pilotVersion)
		return pilotVersion, nil
	}

	chartVersion := getTargetVersionFromVersionInChartDefinition(istioHelmChart)
	if chartVersion != "" {
		logger.Debugf("Resolved target Istio version: %s from Chart definition", chartVersion)
		return chartVersion, nil
	}

	return "", errors.New("Target Istio version could not be found neither in Chart.yaml nor in helm values")
}

func getTargetProxyV2PrefixFromIstioChart(workspace chart.Factory, branch string, istioChart string, logger *zap.SugaredLogger) (string, error) {
	ws, err := workspace.Get(branch)
	if err != nil {
		return "", err
	}

	istioHelmChart, err := loader.Load(filepath.Join(ws.ResourceDir, istioChart))
	if err != nil {
		return "", err
	}

	istioValuesRegistryPath, istioValuesDirectory, err := getTargetProxyV2PrefixFromIstioValues(istioHelmChart)
	if err != nil {
		return "", errors.New("Could not resolve target proxyV2 Istio prefix from values")
	}

	prefix := fmt.Sprintf("%s/%s", istioValuesRegistryPath, istioValuesDirectory)
	logger.Debugf("Resolved target Istio prefix: %s from istio values.yaml", prefix)
	return prefix, nil
}

func getTargetVersionFromVersionInChartDefinition(helmChart *helmChart.Chart) string {
	return helmChart.Metadata.Version
}

func getTargetVersionFromPilotInChartValues(helmChart *helmChart.Chart) (string, error) {
	mapAsJSON, err := json.Marshal(helmChart.Values)
	if err != nil {
		return "", err
	}

	var chartValues chartValues
	err = json.Unmarshal(mapAsJSON, &chartValues)
	if err != nil {
		return "", err
	}

	return chartValues.Global.Images.IstioPilot.Version, nil
}

func getTargetProxyV2PrefixFromIstioValues(istioHelmChart *helmChart.Chart) (string, string, error) {
	mapAsJSON, err := json.Marshal(istioHelmChart.Values)
	if err != nil {
		return "", "", err
	}
	var chartValues chartValues

	err = json.Unmarshal(mapAsJSON, &chartValues)
	if err != nil {
		return "", "", err
	}
	containerRegistryPath := chartValues.Global.Images.IstioProxyV2.ContainerRegistryPath
	directory := chartValues.Global.Images.IstioProxyV2.Directory

	return containerRegistryPath, directory, nil
}

func getVersionFromJSON(versionType VersionType, json IstioVersionOutput) string {
	switch versionType {
	case "client":
		return json.ClientVersion.Version
	case "pilot":
		if len(json.MeshVersion) > 0 {
			return json.MeshVersion[0].Info.Version
		}
		return ""
	default:
		return ""
	}
}

func getUniqueVersionsFromJSON(versionType VersionType, json IstioVersionOutput) map[string]bool {
	switch versionType {
	case "dataPlane":
		if len(json.DataPlaneVersion) > 0 {
			versions := map[string]bool{}
			for _, dpVersion := range json.DataPlaneVersion {
				if _, ok := versions[dpVersion.IstioVersion]; !ok {
					versions[dpVersion.IstioVersion] = true
				}
			}
			return versions
		}
		return map[string]bool{}
	default:
		return map[string]bool{}
	}
}

func mapVersionToStruct(versionOutput []byte, targetVersion string, targetDirectory string) (IstioStatus, error) {
	if len(versionOutput) == 0 {
		return IstioStatus{}, errors.New("the result of the version command is empty")
	}

	if index := bytes.IndexRune(versionOutput, '{'); index != 0 {
		versionOutput = versionOutput[bytes.IndexRune(versionOutput, '{'):]
	}

	var version IstioVersionOutput
	err := json.Unmarshal(versionOutput, &version)

	if err != nil {
		return IstioStatus{}, err
	}

	return IstioStatus{
		ClientVersion:     getVersionFromJSON("client", version),
		TargetVersion:     targetVersion,
		TargetPrefix:      targetDirectory,
		PilotVersion:      getVersionFromJSON("pilot", version),
		DataPlaneVersions: getUniqueVersionsFromJSON("dataPlane", version),
	}, nil
}

func isSidecarMigrationEnabled(workspace chart.Factory, branch string, istioChart string) (option bool, isSet bool, err error) {
	ws, err := workspace.Get(branch)
	if err != nil {
		return false, false, err
	}

	istioHelmChart, err := loader.Load(filepath.Join(ws.ResourceDir, istioChart))
	if err != nil {
		return false, false, err
	}

	mapAsJSON, err := json.Marshal(istioHelmChart.Values)
	if err != nil {
		return false, false, err
	}
	var chartValues chartValues

	err = json.Unmarshal(mapAsJSON, &chartValues)
	if err != nil {
		return false, false, err
	}
	option = chartValues.Global.SidecarMigration

	isSet = false
	var rawValues map[string]map[string]interface{}
	err = json.Unmarshal(mapAsJSON, &rawValues)
	if err != nil {
		return false, false, err
	}
	if global, isGlobalSet := rawValues["global"]; isGlobalSet {
		if _, isSidecarMigrationSet := global["sidecarMigration"]; isSidecarMigrationSet {
			isSet = true
		}
	}

	return option, isSet, nil
}

func IsSidecarInjectionNamespacesByDefaultEnabled(workspace chart.Factory, branch string, istioChart string) (enableNamespacesByDefault bool, err error) {
	ws, err := workspace.Get(branch)
	if err != nil {
		return false, err
	}

	istioHelmChart, err := loader.Load(filepath.Join(ws.ResourceDir, istioChart))
	if err != nil {
		return false, err
	}

	mapAsJSON, err := json.Marshal(istioHelmChart.Values)
	if err != nil {
		return false, err
	}
	var chartValues chartValues

	err = json.Unmarshal(mapAsJSON, &chartValues)
	if err != nil {
		return false, err
	}
	enableNamespacesByDefault = chartValues.HelmValues.SidecarInjectorWebhook.EnableNamespacesByDefault

	return enableNamespacesByDefault, nil
}

func getInstalledIstioVersion(provider clientset.Provider, kubeConfig string, gatherer data.Gatherer, logger *zap.SugaredLogger) (string, error) {
	kubeClient, err := provider.RetrieveFrom(kubeConfig, logger)
	if err != nil {
		logger.Error("Could not retrieve KubeClient from Kubeconfig!")
		return "", err
	}
	retryOpts := []avastretry.Option{
		avastretry.Delay(delayBetweenRetries),
		avastretry.Attempts(uint(retriesCount)),
		avastretry.DelayType(avastretry.FixedDelay),
	}

	version, err := gatherer.GetInstalledIstioVersion(kubeClient, retryOpts, logger)
	if err != nil {
		return "", err
	}

	return version, nil
}
