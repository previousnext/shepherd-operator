package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	osv1client "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	EnvMysqlHost     = "DATABASE_HOST"
	EnvMysqlUsername = "DATABASE_USER"
	EnvMysqlDatabase = "DATABASE_NAME"
	EnvMysqlPort     = "DATABASE_PORT"
)

var (
	kubeconfig *string
	namespace *string
	dryRun *bool
)

func main() {
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	namespace = flag.String("namespace", "", "the namespace to migrate")
	dryRun = flag.Bool("dry-run", false, "when provided script will not modify objects")
	flag.Parse()
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	client, err := kubernetes.NewForConfig(config)
	routeClient, err := routev1client.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	osclient, err := osv1client.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	if *dryRun {
		fmt.Println("------------ DRY RUN ------------")
		fmt.Println("-- No objects will be modified --")
		fmt.Println("---------------------------------")
	}

	fmt.Println("Starting migration")

	deployConfigs, err := osclient.DeploymentConfigs(*namespace).List(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	var secretsChecked []string
	var servicesChecked []string
	var routesChecked []string
	var pvcsChecked []string
	for _, dc := range deployConfigs.Items {
		pod := dc.Spec.Template
		fmt.Println("- Checking DeployConfig", dc.Name)
		if _, found := pod.ObjectMeta.GetLabels()["app"]; !found {
			fmt.Println("  Skipping Pod", dc.Name, "as it doesn't have an app label")
			continue
		}
		appLabel := pod.ObjectMeta.GetLabels()["app"]
		fmt.Println("- Checking Secret for", dc.Name)
		secret, err := migrateSecretData(client, pod, appLabel, secretsChecked)
		if err != nil {
			panic(err)
		}
		if secret != nil {
			secretsChecked = append(secretsChecked, secret.Name)
		}

		fmt.Println("- Checking Service for", dc.Name)
		service, err := migrateServiceLabel(client, pod, appLabel, servicesChecked)
		if err != nil {
			panic(err)
		}
		if service != nil {
			servicesChecked = append(servicesChecked, service.Name)
		}

		fmt.Println("- Checking Route for", dc.Name)
		route, err := migrateRouteLabel(routeClient, pod, appLabel, routesChecked)
		if err != nil {
			panic(err)
		}
		if route != nil {
			routesChecked = append(routesChecked, route.Name)
		}

		fmt.Println("- Checking shared PVC for", dc.Name)
		pvc, err := migratePvcLabel(client, pod, appLabel, pvcsChecked)
		if err != nil {
			panic(err)
		}
		if pvc != nil {
			pvcsChecked = append(pvcsChecked, pvc.Name)
		}

		// Check if changes above triggered a redeploy and wait for it to complete.
		for {
			dcRefresh, err := osclient.DeploymentConfigs(*namespace).Get(dc.Name, metav1.GetOptions{})
			if err != nil {
				fmt.Println(err.Error())
				break;
			}

			if dc.Generation == dcRefresh.Generation {
				break;
			}

			if dcRefresh.Status.ObservedGeneration == dcRefresh.Generation {
				fmt.Println("Deployment completed!")
				break;
			}

			fmt.Println("  ... waiting for deployment to complete ...")
			time.Sleep(5 * time.Second)
		}

	}
	fmt.Println("Completed migration")
}

// migrateSecretData migrates data from env vars on Pods into secrets.
func migrateSecretData(client *kubernetes.Clientset, pod *v1.PodTemplateSpec, appLabel string, secretsChecked []string) (*v1.Secret, error) {
	name := pod.Spec.Containers[0].Name
	requiredEnv := []string{
		EnvMysqlHost,
		EnvMysqlUsername,
		EnvMysqlDatabase,
		EnvMysqlPort,
	}
	secret, err := client.CoreV1().Secrets(*namespace).Get(appLabel, metav1.GetOptions{})
	if err != nil {
		fmt.Println("  Skipping Pod", name, "as it doesn't have a Secret matching its app label")
		return nil, nil
	}
	if Contains(secretsChecked, secret.Name) {
		fmt.Println("  Skipping Pod", name, "as the secret has already been checked")
		return nil, nil
	}
	fmt.Println("  Pod", name, "found with secret that may need updating")

	container := pod.Spec.Containers[0]
	foundEnvs := make(map[string]string, len(requiredEnv))
	for _, env := range container.Env {
		if Contains(requiredEnv, env.Name) {
			foundEnvs[env.Name] = env.Value
		}
	}
	if len(requiredEnv) != len(foundEnvs) {
		fmt.Println("  Skipping Pod", name, "as it doesn't have all required env vars")
		return secret, nil
	}

	fmt.Println("  Checking Secret", secret.Name, "if it needs an update")
	needsUpdate := false
	for key, value := range foundEnvs {
		if _, found := secret.Data[key]; !found {
			needsUpdate = true
			if len(secret.StringData) == 0 {
				secret.StringData = make(map[string]string, len(secret.Data))
			}
			secret.StringData[key] = value
			fmt.Println("    Setting", key, "in Secret", secret.Name)
		}
	}
	if _, found := secret.ObjectMeta.GetLabels()["app"]; !found {
		fmt.Println("  Secret", secret.Name, "doesn't have an app label, setting it")
		labels := secret.GetLabels()
		if len(labels) == 0 {
			labels = make(map[string]string)
		}
		labels["app"] = appLabel
		secret.SetLabels(labels)
		needsUpdate = true
	}
	if needsUpdate {
		fmt.Println("  Updating Secret", secret.Name, "with db creds")
		if !*dryRun {
			secret, err = client.CoreV1().Secrets(secret.Namespace).Update(secret)
			if err != nil {
				return nil, err
			}
		}

	} else {
		fmt.Println("  Secret", secret.Name, "didn't need updating")
	}
	return secret, nil
}

// migrateServiceLabel migrates services to add the app label.
func migrateServiceLabel(client *kubernetes.Clientset, pod *v1.PodTemplateSpec, appLabel string, servicesChecked []string) (*v1.Service, error) {
	// Pass in the container name here as redis pods have services too but have the same app label
	// as the web pod.
	serviceName := pod.Spec.Containers[0].Name
	service, err := client.CoreV1().Services(*namespace).Get(serviceName, metav1.GetOptions{})
	if err != nil {
		fmt.Println("Skipping Pod", serviceName, "as it doesn't have a Service matching container name")
		return nil, nil
	}
	if Contains(servicesChecked, service.Name) {
		fmt.Println("Skipping Service", service.Name, "as it has already been checked")
		return nil, nil
	}
	if _, found := service.ObjectMeta.GetLabels()["app"]; found {
		fmt.Println("Skipping Service", service.Name, "as it already has an app label")
		return service, nil
	}
	fmt.Println("Updating Service", service.Name, "with app label")
	labels := service.GetLabels()
	if len(labels) == 0 {
		labels = make(map[string]string)
	}
	labels["app"] = appLabel
	service.SetLabels(labels)

	if !*dryRun {
		service, err = client.CoreV1().Services(service.Namespace).Update(service)
		if err != nil {
			return nil, err
		}
	}

	return service, nil
}

// migratePvcLabel migrates pvcs to add the app label.
func migratePvcLabel(client *kubernetes.Clientset, pod *v1.PodTemplateSpec, appLabel string, pvcsChecked []string) (*v1.PersistentVolumeClaim, error) {
	name := pod.Spec.Containers[0].Name
	for _, volume := range pod.Spec.Volumes {
		// Not a PVC claim, skip.
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		// Not the shared PVC claim, skip.
		if volume.PersistentVolumeClaim.ClaimName != fmt.Sprintf("%s-shared", appLabel) {
			continue
		}
		pvc, err := client.CoreV1().PersistentVolumeClaims(*namespace).Get(volume.PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
		if err != nil {
			fmt.Println("Skipping Pod", name, "as there was an error loading its shared pvc")
			return nil, nil
		}
		if Contains(pvcsChecked, pvc.Name) {
			fmt.Println("Skipping PVC", pvc.Name, "as it has already been checked")
			return nil, nil
		}
		if _, found := pvc.ObjectMeta.GetLabels()["app"]; found {
			fmt.Println("Skipping PVC", pvc.Name, "as it already has an app label")
			return pvc, nil
		}
		fmt.Println("Updating PVC", pvc.Name, "with app label")
		labels := pvc.GetLabels()
		if len(labels) == 0 {
			labels = make(map[string]string)
		}
		labels["app"] = appLabel
		pvc.SetLabels(labels)
		if !*dryRun {
			pvc, err = client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Update(pvc)

			if err != nil {
				return nil, err
			}
		}
		return pvc, nil
	}
	return nil, nil
}

// migrateRouteLabel migrates routes to add the app label.
func migrateRouteLabel(client *routev1client.RouteV1Client, pod *v1.PodTemplateSpec, appLabel string, routesChecked []string) (*routev1.Route, error) {
	name := pod.Spec.Containers[0].Name
	route, err := client.Routes(*namespace).Get(appLabel, metav1.GetOptions{})
	if err != nil {
		fmt.Println("  Skipping Pod", name, "as it doesn't have a Route matching its app label")
		return nil, nil
	}
	if Contains(routesChecked, route.Name) {
		fmt.Println("  Skipping Route", route.Name, "as it has already been checked")
		return nil, nil
	}
	if _, found := route.ObjectMeta.GetLabels()["app"]; found {
		fmt.Println("  Skipping Route", route.Name, "as it already has an app label")
		return route, nil
	}
	fmt.Println("  Updating Route", route.Name, "with app label")
	labels := route.GetLabels()
	if len(labels) == 0 {
		labels = make(map[string]string)
	}
	labels["app"] = appLabel
	route.SetLabels(labels)
	if !*dryRun {
		route, err = client.Routes(route.Namespace).Update(route)
		if err != nil {
			return nil, err
		}
	}
	return route, nil
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE")
}

func Contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}

	return false
}
