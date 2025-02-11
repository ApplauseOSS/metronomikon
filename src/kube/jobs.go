package kube

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Delete specific CronJob
// Returns nil and error when job doesn't exist
// Return job and error when deleting job failed
// Return job and nil when deletion succeeded
func DeleteCronJob(namespace string, name string) (*batchv1.CronJob, error) {
	job, err := GetCronJob(namespace, name)
	if err != nil {
		return nil, err
	}
	err = client.BatchV1().CronJobs(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return job, err
	}
	return job, nil
}

// Return all CronJobs in a namespace
func GetCronJobs(namespace string) ([]batchv1.CronJob, error) {
	jobs, err := client.BatchV1().CronJobs(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not list CronJobs: %s", err)
	}
	return jobs.Items, nil
}

// Return specific CronJob
func GetCronJob(namespace string, name string) (*batchv1.CronJob, error) {
	job, err := client.BatchV1().CronJobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get CronJob: %s", err)
	}
	return job, nil
}

// GetPods returns array of pods where owner kind is given
func GetPods(namespace string, ownerKind string) ([]corev1.Pod, error) {
	pods, err := client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get pods from job: %s", err)
	}
	result := []corev1.Pod{}
	for _, pod := range pods.Items {
		for _, ownerReference := range pod.ObjectMeta.GetOwnerReferences() {
			if ownerReference.Kind == ownerKind {
				result = append(result, pod)
				break
			}
		}
	}
	return result, nil
}

func GetJobsFromCronJob(namespace string, cronJobName string) ([]batchv1.Job, error) {
	jobs, err := client.BatchV1().Jobs(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get jobs from cronjob: %s", err)
	}
	res := []batchv1.Job{}
	for _, job := range jobs.Items {
		for _, ownerReference := range job.ObjectMeta.GetOwnerReferences() {
			if ownerReference.Name == cronJobName {
				res = append(res, job)
				break
			}
		}
	}
	return res, nil
}

// Create Job from CronJob
func CreateJobFromCronjob(cronJob *batchv1.CronJob) (*batchv1.Job, error) {
	// This duplicates the logic used by kubectl to create a Job from a CronJob
	annotations := make(map[string]string)
	annotations["cronjob.kubernetes.io/instantiate"] = "manual"
	for k, v := range cronJob.Spec.JobTemplate.Annotations {
		annotations[k] = v
	}

	jobDef := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{APIVersion: batchv1.SchemeGroupVersion.String(), Kind: "Job"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%d", cronJob.ObjectMeta.Name, time.Now().Unix()),
			Annotations: annotations,
			Labels:      cronJob.Spec.JobTemplate.Labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cronJob, appsv1.SchemeGroupVersion.WithKind("CronJob")),
			},
		},
		Spec: cronJob.Spec.JobTemplate.Spec,
	}

	if job, err := client.BatchV1().Jobs(cronJob.ObjectMeta.Namespace).Create(context.TODO(), jobDef, metav1.CreateOptions{}); err != nil {
		return nil, err
	} else {
		return job, nil
	}
}

// Return specific Job
func GetJob(namespace string, name string) (*batchv1.Job, error) {
	job, err := client.BatchV1().Jobs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get Job: %s", err)
	}
	return job, nil
}
