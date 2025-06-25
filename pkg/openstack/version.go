package openstack

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	missingImageDefault string
)

// InitializeOpenStackVersionImageDefaults - initializes OpenStackVersion CR with default container images
func InitializeOpenStackVersionImageDefaults(ctx context.Context, envImages map[string]*string) *corev1beta1.ContainerDefaults {
	Log := GetLogger(ctx)

	defaults := &corev1beta1.ContainerDefaults{}

	d := reflect.ValueOf(defaults).Elem()
	for key, val := range envImages {
		//Log.Info(fmt.Sprintf("Initialize OpenStackVersion Image Defaults: %s", key))

		r := regexp.MustCompile(`[A-Za-z0-9]+`)
		matches := r.FindAllString(key, -1)
		fieldName := ""
		// only match related image strings
		if matches[0] == "RELATED" && matches[1] == "IMAGE" {
			// exclude prefix and suffix fields (2 and 2 each respectively)
			// first 2 fields are "RELATED" and "IMAGE"
			// last 2 fields are "URL" and "DEFAULT"
			for i := 2; i < len(matches)-2; i++ {
				fieldName += strings.ToUpper(matches[i])[0:1]
				fieldName += strings.ToLower(matches[i])[1:]
			}
			// format API so we adhere to go linting standards
			fieldName = strings.Replace(fieldName, "Api", "API", -1)
		}
		//Log.Info(fmt.Sprintf("Initialize Field name: %s", fieldName))
		field := d.FieldByName(fieldName)
		if field.IsValid() && field.CanSet() {
			field.Set(reflect.ValueOf(val))
		} else {
			Log.Info(fmt.Sprintf("Field not found: %s", fieldName))
		}
	}
	Log.Info("Initialize OpenStackVersion Cinder/Manila:")
	if envImages["RELATED_IMAGE_CINDER_VOLUME_IMAGE_URL_DEFAULT"] != nil {
		defaults.CinderVolumeImage = envImages["RELATED_IMAGE_CINDER_VOLUME_IMAGE_URL_DEFAULT"]
	}
	if envImages["RELATED_IMAGE_MANILA_SHARE_IMAGE_URL_DEFAULT"] != nil {
		defaults.ManilaShareImage = envImages["RELATED_IMAGE_MANILA_SHARE_IMAGE_URL_DEFAULT"]
	}
	// this is shared with the infra-operator (for dnsmasq), avoiding two RELATED_IMAGES
	// with the same value fixes bundle validation warnings
	if envImages["RELATED_IMAGE_NEUTRON_API_IMAGE_URL_DEFAULT"] != nil {
		defaults.InfraDnsmasqImage = envImages["RELATED_IMAGE_NEUTRON_API_IMAGE_URL_DEFAULT"]
	}
	// custom TEST_ images which aren't released downstream
	if envImages["TEST_TOBIKO_IMAGE_URL_DEFAULT"] != nil {
		defaults.TestTobikoImage = envImages["TEST_TOBIKO_IMAGE_URL_DEFAULT"]
	}
	if envImages["TEST_ANSIBLETEST_IMAGE_URL_DEFAULT"] != nil {
		defaults.TestAnsibletestImage = envImages["TEST_ANSIBLETEST_IMAGE_URL_DEFAULT"]
	}
	if envImages["TEST_HORIZONTEST_IMAGE_URL_DEFAULT"] != nil {
		defaults.TestHorizontestImage = envImages["TEST_HORIZONTEST_IMAGE_URL_DEFAULT"]
	}

	Log.Info("Initialize OpenStackVersion return defaults")
	return defaults

}

// getImg return val1 if set, otherwise return val2
func getImg(val1 *string, val2 *string) *string {
	if val1 != nil {
		return val1
	}
	return val2

}

// GetContainerImages - initializes OpenStackVersion CR with default container images
func GetContainerImages(defaults *corev1beta1.ContainerDefaults, instance corev1beta1.OpenStackVersion) corev1beta1.ContainerImages {

	containerImages := corev1beta1.ContainerImages{
		CinderVolumeImages:   instance.Spec.CustomContainerImages.CinderVolumeImages,
		ManilaShareImages:    instance.Spec.CustomContainerImages.ManilaShareImages,
		CeilometerProxyImage: getImg(instance.Spec.CustomContainerImages.ApacheImage, defaults.ApacheImage),
		OctaviaApacheImage:   getImg(instance.Spec.CustomContainerImages.ApacheImage, defaults.ApacheImage),
		ContainerTemplate: corev1beta1.ContainerTemplate{
			AgentImage:                        getImg(instance.Spec.CustomContainerImages.AgentImage, defaults.AgentImage),
			AnsibleeeImage:                    getImg(instance.Spec.CustomContainerImages.AnsibleeeImage, defaults.AnsibleeeImage),
			AodhAPIImage:                      getImg(instance.Spec.CustomContainerImages.AodhAPIImage, defaults.AodhAPIImage),
			AodhEvaluatorImage:                getImg(instance.Spec.CustomContainerImages.AodhEvaluatorImage, defaults.AodhEvaluatorImage),
			AodhListenerImage:                 getImg(instance.Spec.CustomContainerImages.AodhListenerImage, defaults.AodhListenerImage),
			AodhNotifierImage:                 getImg(instance.Spec.CustomContainerImages.AodhNotifierImage, defaults.AodhNotifierImage),
			ApacheImage:                       getImg(instance.Spec.CustomContainerImages.ApacheImage, defaults.ApacheImage),
			BarbicanAPIImage:                  getImg(instance.Spec.CustomContainerImages.BarbicanAPIImage, defaults.BarbicanAPIImage),
			BarbicanKeystoneListenerImage:     getImg(instance.Spec.CustomContainerImages.BarbicanKeystoneListenerImage, defaults.BarbicanKeystoneListenerImage),
			BarbicanWorkerImage:               getImg(instance.Spec.CustomContainerImages.BarbicanWorkerImage, defaults.BarbicanWorkerImage),
			CeilometerCentralImage:            getImg(instance.Spec.CustomContainerImages.CeilometerCentralImage, defaults.CeilometerCentralImage),
			CeilometerComputeImage:            getImg(instance.Spec.CustomContainerImages.CeilometerComputeImage, defaults.CeilometerComputeImage),
			CeilometerIpmiImage:               getImg(instance.Spec.CustomContainerImages.CeilometerIpmiImage, defaults.CeilometerIpmiImage),
			CeilometerNotificationImage:       getImg(instance.Spec.CustomContainerImages.CeilometerNotificationImage, defaults.CeilometerNotificationImage),
			CeilometerSgcoreImage:             getImg(instance.Spec.CustomContainerImages.CeilometerSgcoreImage, defaults.CeilometerSgcoreImage),
			CeilometerMysqldExporterImage:     getImg(instance.Spec.CustomContainerImages.CeilometerMysqldExporterImage, defaults.CeilometerMysqldExporterImage),
			CinderAPIImage:                    getImg(instance.Spec.CustomContainerImages.CinderAPIImage, defaults.CinderAPIImage),
			CinderBackupImage:                 getImg(instance.Spec.CustomContainerImages.CinderBackupImage, defaults.CinderBackupImage),
			CinderSchedulerImage:              getImg(instance.Spec.CustomContainerImages.CinderSchedulerImage, defaults.CinderSchedulerImage),
			DesignateAPIImage:                 getImg(instance.Spec.CustomContainerImages.DesignateAPIImage, defaults.DesignateAPIImage),
			DesignateBackendbind9Image:        getImg(instance.Spec.CustomContainerImages.DesignateBackendbind9Image, defaults.DesignateBackendbind9Image),
			DesignateCentralImage:             getImg(instance.Spec.CustomContainerImages.DesignateCentralImage, defaults.DesignateCentralImage),
			DesignateMdnsImage:                getImg(instance.Spec.CustomContainerImages.DesignateMdnsImage, defaults.DesignateMdnsImage),
			DesignateProducerImage:            getImg(instance.Spec.CustomContainerImages.DesignateProducerImage, defaults.DesignateProducerImage),
			DesignateUnboundImage:             getImg(instance.Spec.CustomContainerImages.DesignateUnboundImage, defaults.DesignateUnboundImage),
			DesignateWorkerImage:              getImg(instance.Spec.CustomContainerImages.DesignateWorkerImage, defaults.DesignateWorkerImage),
			EdpmFrrImage:                      getImg(instance.Spec.CustomContainerImages.EdpmFrrImage, defaults.EdpmFrrImage),
			EdpmIscsidImage:                   getImg(instance.Spec.CustomContainerImages.EdpmIscsidImage, defaults.EdpmIscsidImage),
			EdpmLogrotateCrondImage:           getImg(instance.Spec.CustomContainerImages.EdpmLogrotateCrondImage, defaults.EdpmLogrotateCrondImage),
			EdpmMultipathdImage:               getImg(instance.Spec.CustomContainerImages.EdpmMultipathdImage, defaults.EdpmMultipathdImage),
			EdpmNeutronDhcpAgentImage:         getImg(instance.Spec.CustomContainerImages.EdpmNeutronDhcpAgentImage, defaults.EdpmNeutronDhcpAgentImage),
			EdpmNeutronMetadataAgentImage:     getImg(instance.Spec.CustomContainerImages.EdpmNeutronMetadataAgentImage, defaults.EdpmNeutronMetadataAgentImage),
			EdpmNeutronOvnAgentImage:          getImg(instance.Spec.CustomContainerImages.EdpmNeutronOvnAgentImage, defaults.EdpmNeutronOvnAgentImage),
			EdpmNeutronSriovAgentImage:        getImg(instance.Spec.CustomContainerImages.EdpmNeutronSriovAgentImage, defaults.EdpmNeutronSriovAgentImage),
			EdpmOvnBgpAgentImage:              getImg(instance.Spec.CustomContainerImages.EdpmOvnBgpAgentImage, defaults.EdpmOvnBgpAgentImage),
			EdpmNodeExporterImage:             getImg(instance.Spec.CustomContainerImages.EdpmNodeExporterImage, defaults.EdpmNodeExporterImage),
			EdpmKeplerImage:                   getImg(instance.Spec.CustomContainerImages.EdpmKeplerImage, defaults.EdpmKeplerImage),
			EdpmPodmanExporterImage:           getImg(instance.Spec.CustomContainerImages.EdpmPodmanExporterImage, defaults.EdpmPodmanExporterImage),
			EdpmOpenstackNetworkExporterImage: getImg(instance.Spec.CustomContainerImages.EdpmOpenstackNetworkExporterImage, defaults.EdpmOpenstackNetworkExporterImage),
			GlanceAPIImage:                    getImg(instance.Spec.CustomContainerImages.GlanceAPIImage, defaults.GlanceAPIImage),
			HeatAPIImage:                      getImg(instance.Spec.CustomContainerImages.HeatAPIImage, defaults.HeatAPIImage),
			HeatCfnapiImage:                   getImg(instance.Spec.CustomContainerImages.HeatCfnapiImage, defaults.HeatCfnapiImage),
			HeatEngineImage:                   getImg(instance.Spec.CustomContainerImages.HeatEngineImage, defaults.HeatEngineImage),
			HorizonImage:                      getImg(instance.Spec.CustomContainerImages.HorizonImage, defaults.HorizonImage),
			InfraDnsmasqImage:                 getImg(instance.Spec.CustomContainerImages.InfraDnsmasqImage, defaults.InfraDnsmasqImage),
			InfraMemcachedImage:               getImg(instance.Spec.CustomContainerImages.InfraMemcachedImage, defaults.InfraMemcachedImage),
			InfraRedisImage:                   getImg(instance.Spec.CustomContainerImages.InfraRedisImage, defaults.InfraRedisImage),
			IronicAPIImage:                    getImg(instance.Spec.CustomContainerImages.IronicAPIImage, defaults.IronicAPIImage),
			IronicConductorImage:              getImg(instance.Spec.CustomContainerImages.IronicConductorImage, defaults.IronicConductorImage),
			IronicInspectorImage:              getImg(instance.Spec.CustomContainerImages.IronicInspectorImage, defaults.IronicInspectorImage),
			IronicNeutronAgentImage:           getImg(instance.Spec.CustomContainerImages.IronicNeutronAgentImage, defaults.IronicNeutronAgentImage),
			IronicPxeImage:                    getImg(instance.Spec.CustomContainerImages.IronicPxeImage, defaults.IronicPxeImage),
			IronicPythonAgentImage:            getImg(instance.Spec.CustomContainerImages.IronicPythonAgentImage, defaults.IronicPythonAgentImage),
			KeystoneAPIImage:                  getImg(instance.Spec.CustomContainerImages.KeystoneAPIImage, defaults.KeystoneAPIImage),
			KsmImage:                          getImg(instance.Spec.CustomContainerImages.KsmImage, defaults.KsmImage),
			ManilaAPIImage:                    getImg(instance.Spec.CustomContainerImages.ManilaAPIImage, defaults.ManilaAPIImage),
			ManilaSchedulerImage:              getImg(instance.Spec.CustomContainerImages.ManilaSchedulerImage, defaults.ManilaSchedulerImage),
			MariadbImage:                      getImg(instance.Spec.CustomContainerImages.MariadbImage, defaults.MariadbImage),
			NetUtilsImage:                     getImg(instance.Spec.CustomContainerImages.NetUtilsImage, defaults.NetUtilsImage),
			NeutronAPIImage:                   getImg(instance.Spec.CustomContainerImages.NeutronAPIImage, defaults.NeutronAPIImage),
			NovaAPIImage:                      getImg(instance.Spec.CustomContainerImages.NovaAPIImage, defaults.NovaAPIImage),
			NovaComputeImage:                  getImg(instance.Spec.CustomContainerImages.NovaComputeImage, defaults.NovaComputeImage),
			NovaConductorImage:                getImg(instance.Spec.CustomContainerImages.NovaConductorImage, defaults.NovaConductorImage),
			NovaNovncImage:                    getImg(instance.Spec.CustomContainerImages.NovaNovncImage, defaults.NovaNovncImage),
			NovaSchedulerImage:                getImg(instance.Spec.CustomContainerImages.NovaSchedulerImage, defaults.NovaSchedulerImage),
			OctaviaAPIImage:                   getImg(instance.Spec.CustomContainerImages.OctaviaAPIImage, defaults.OctaviaAPIImage),
			OctaviaHealthmanagerImage:         getImg(instance.Spec.CustomContainerImages.OctaviaHealthmanagerImage, defaults.OctaviaHealthmanagerImage),
			OctaviaHousekeepingImage:          getImg(instance.Spec.CustomContainerImages.OctaviaHousekeepingImage, defaults.OctaviaHousekeepingImage),
			OctaviaWorkerImage:                getImg(instance.Spec.CustomContainerImages.OctaviaWorkerImage, defaults.OctaviaWorkerImage),
			OctaviaRsyslogImage:               getImg(instance.Spec.CustomContainerImages.OctaviaRsyslogImage, defaults.OctaviaRsyslogImage),
			OpenstackClientImage:              getImg(instance.Spec.CustomContainerImages.OpenstackClientImage, defaults.OpenstackClientImage),
			OsContainerImage:                  getImg(instance.Spec.CustomContainerImages.OsContainerImage, defaults.OsContainerImage),
			OvnControllerImage:                getImg(instance.Spec.CustomContainerImages.OvnControllerImage, defaults.OvnControllerImage),
			OvnControllerOvsImage:             getImg(instance.Spec.CustomContainerImages.OvnControllerOvsImage, defaults.OvnControllerOvsImage),
			OvnNbDbclusterImage:               getImg(instance.Spec.CustomContainerImages.OvnNbDbclusterImage, defaults.OvnNbDbclusterImage),
			OvnNorthdImage:                    getImg(instance.Spec.CustomContainerImages.OvnNorthdImage, defaults.OvnNorthdImage),
			OvnSbDbclusterImage:               getImg(instance.Spec.CustomContainerImages.OvnSbDbclusterImage, defaults.OvnSbDbclusterImage),
			PlacementAPIImage:                 getImg(instance.Spec.CustomContainerImages.PlacementAPIImage, defaults.PlacementAPIImage),
			RabbitmqImage:                     getImg(instance.Spec.CustomContainerImages.RabbitmqImage, defaults.RabbitmqImage),
			SwiftAccountImage:                 getImg(instance.Spec.CustomContainerImages.SwiftAccountImage, defaults.SwiftAccountImage),
			SwiftContainerImage:               getImg(instance.Spec.CustomContainerImages.SwiftContainerImage, defaults.SwiftContainerImage),
			SwiftObjectImage:                  getImg(instance.Spec.CustomContainerImages.SwiftObjectImage, defaults.SwiftObjectImage),
			SwiftProxyImage:                   getImg(instance.Spec.CustomContainerImages.SwiftProxyImage, defaults.SwiftProxyImage),
			TelemetryNodeExporterImage:        getImg(instance.Spec.CustomContainerImages.TelemetryNodeExporterImage, defaults.TelemetryNodeExporterImage),
			TestTempestImage:                  getImg(instance.Spec.CustomContainerImages.TestTempestImage, defaults.TestTempestImage),
			TestTobikoImage:                   getImg(instance.Spec.CustomContainerImages.TestTobikoImage, defaults.TestTobikoImage),
			TestHorizontestImage:              getImg(instance.Spec.CustomContainerImages.TestHorizontestImage, defaults.TestHorizontestImage),
			TestAnsibletestImage:              getImg(instance.Spec.CustomContainerImages.TestAnsibletestImage, defaults.TestAnsibletestImage),
		}}
	if containerImages.CinderVolumeImages == nil {
		containerImages.CinderVolumeImages = make(map[string]*string)
	}
	if containerImages.ManilaShareImages == nil {
		containerImages.ManilaShareImages = make(map[string]*string)
	}
	containerImages.CinderVolumeImages["default"] = defaults.CinderVolumeImage
	containerImages.ManilaShareImages["default"] = defaults.ManilaShareImage
	return containerImages
}

// InitializeOpenStackVersionImageDefaults - initializes OpenStackVersion CR with default container images
func InitializeOpenStackVersionServiceDefaults(ctx context.Context) *corev1beta1.ServiceDefaults {
	Log := GetLogger(ctx)
	Log.Info("Initialize OpenStackVersion Service Defaults")

	defaults := &corev1beta1.ServiceDefaults{}

	// NOTE: defaults change over time, older OpenStackVersion defaults would default to false (FR2 for example),
	// but get set to true here for FR3 available versions and thus provide a way for services to migrate
	// to new deployment topologies
	trueString := "true"
	defaults.GlanceWsgi = &trueString // all new glance deployments use WSGI by default (FR3 and later)

	return defaults
}

// ReconcileVersion - reconciles OpenStackVersion CR
func ReconcileVersion(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, *corev1beta1.OpenStackVersion, error) {
	version := &corev1beta1.OpenStackVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}

	Log := GetLogger(ctx)

	// return if OpenStackVersion CR already exists
	if err := helper.GetClient().Get(ctx, types.NamespacedName{
		Name:      instance.Name,
		Namespace: instance.Namespace,
	},
		version); err == nil {
		Log.Info(fmt.Sprintf("OpenStackVersion found. Name: %s", version.Name))
	} else {
		Log.Info(fmt.Sprintf("OpenStackVersion does not exist. Creating: %s", version.Name))
	}

	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), version, func() error {
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), version, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return ctrl.Result{}, nil, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("OpenStackVersion %s - %s", version.Name, op))
	}

	return ctrl.Result{}, version, nil
}

func stringPointersEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// ControlplaneContainerImageMatch - function to compare the ContainerImages on the controlPlane to the OpenStackVersion
// only enabled services are checked
func ControlplaneContainerImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) (bool, []string) {
	failedMatches := []string{}
	if BarbicanImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Barbican")
	}
	if CinderImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Cinder")
	}
	if DesignateImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Designate")
	}
	if DnsmasqImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Dnsmasq")
	}
	if GaleraImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Galera")
	}
	if GlanceImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Glance")
	}
	if HeatImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Heat")
	}
	if HorizonImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Horizon")
	}
	if IronicImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Ironic")
	}
	if KeystoneImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Keystone")
	}
	if ManilaImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Manila")
	}
	if MemcachedImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Memcached")
	}
	if RedisImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Redis")
	}
	if NeutronImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Neutron")
	}
	if NovaImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Nova")
	}
	if OctaviaImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Octavia")
	}
	if ClientImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "OpenstackClient")
	}
	if OVNControllerImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "OVNController")
	}
	if OVNNorthImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "OVNNorth")
	}
	if OVNDbClusterImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "OVNDbCluster")
	}
	if PlacementImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Placement")
	}
	if RabbitmqImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Rabbitmq")
	}
	if SwiftImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Swift")
	}
	if TelemetryImageMatch(ctx, controlPlane, version) {
		failedMatches = append(failedMatches, "Telemetry")
	}

	if len(failedMatches) == 0 {
		return true, nil
	}

	return false, failedMatches
}
