package helpers

import (
	"fmt"
	"strings"
	"time"

	"github.com/applauseoss/metronomikon/config"
	"github.com/applauseoss/metronomikon/kube"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

// MetronomeErrorMessage supports Metronome API fileds when returning error
type GinErrorMessage struct {
	HTTPCode int
	Message  string `json:"message"`
}

// Struct for handling parsing/generation of Metronome job JSON schema
type MetronomeJob struct {
	Id          string               `json:"id"`
	Description string               `json:"description,omitempty"`
	History     *MetronomeJobHistory `json:"history,omitempty"`
	Labels      map[string]string    `json:"labels,omitempty"`
	Run         struct {
		Args      []string `json:"args,omitempty"`
		Artifacts []struct {
			Uri        string `json:"uri"`
			Executable bool   `json:"executable"`
			Extract    bool   `json:"extract"`
			Cache      bool   `json:"cache"`
		} `json:"artifacts,omitempty"`
		Cmd    string  `json:"cmd,omitempty"`
		Cpus   float32 `json:"cpus"`
		Disk   float32 `json:"disk"`
		Docker struct {
			Image string `json:"image"`
		} `json:"docker"`
		Env            map[string]string `json:"env,omitempty"`
		MaxLaunchDelay int               `json:"maxLaunchDelay,omitempty"`
		Mem            float32           `json:"mem"`
		Placement      struct {
			Constraints []struct {
				Attribute string `json:"attribute,omitempty"`
				Operator  string `json:"operator,omitempty"`
				Value     string `json:"value,omitempty"`
			} `json:"constraints,omitempty"`
		} `json:"placement,omitempty"`
		User    string `json:"user,omitempty"`
		Restart struct {
			Policy                string `json:"policy,omitempty"`
			ActiveDeadlineSeconds int    `json:"activeDeadlineSeconds,omitempty"`
		} `json:"restart,omitempty"`
		Volumes []struct {
			ContainerPath string `json:"containerPath,omitempty"`
			HostPath      string `json:"hostPath,omitempty"`
			Mode          string `json:"mode,omitempty"`
		} `json:"volumes,omitempty"`
	} `json:"run"`
}

// MetronomeJobHistory is used for embeding job history into MetronomeJob
type MetronomeJobHistory struct {
	SuccessCount           int                        `json:"successCount"`
	FailureCount           int                        `json:"failureCount"`
	LastSuccessAt          *string                    `json:"lastSuccessAt"`
	LastFailureAt          *string                    `json:"lastFailureAt"`
	SuccessfulFinishedRuns []MetronomeJobHistoryEntry `json:"successfulFinishedRuns"`
	FailedFinishedRuns     []MetronomeJobHistoryEntry `json:"failedFinishedRuns"`
}

// MetronomeJobHistoryEntry represents single metronome job run
type MetronomeJobHistoryEntry struct {
	ID         string   `json:"id"`
	CreatedAt  string   `json:"createdAt"`
	FinishedAt *string  `json:"finishedAt"`
	Tasks      []string `json:"tasks"`
}

type MetronomeJobRun struct {
	CompletedAt *string  `json:"completedAt"` // we use a pointer so that we can get a null in the JSON if not populated
	CreatedAt   string   `json:"createdAt"`
	Id          string   `json:"id"`
	JobId       string   `json:"jobId"`
	Status      string   `json:"status"`
	Tasks       []string `json:"tasks"`
}

// Convert Kubernetes CronJob to Metronome format
func CronJobKubernetesToMetronome(cronJob *batchv1.CronJob) *MetronomeJob {
	ret := &MetronomeJob{}
	cfg := config.GetConfig()
	ret.Id = fmt.Sprintf("%s.%s", cronJob.Namespace, cronJob.Name)
	// Metronome only supports a single container, so we grab the first one
	// XXX: make this configurable?
	container := cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[0]
	ret.Run.Docker.Image = container.Image
	ret.Run.Args = container.Args
	if len(container.Command) > 0 {
		ret.Run.Cmd = strings.Join(container.Command, " ")
	}
	// TODO: pull values from CronJob 'resources'
	ret.Run.Mem = cfg.Metronome.JobDefaults.Memory
	ret.Run.Disk = cfg.Metronome.JobDefaults.Disk
	ret.Run.Cpus = cfg.Metronome.JobDefaults.Cpus
	return ret
}

func anyJobConditionsFailed(conditions []batchv1.JobCondition) bool {
	for _, condition := range conditions {
		if condition.Type == "Failed" && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func firstJobConditionFailedTime(conditions []batchv1.JobCondition) string {
	for _, condition := range conditions {
		if condition.Type == "Failed" && condition.Status == corev1.ConditionTrue {
			return condition.LastTransitionTime.String()
		}
	}
	return "0"
}

// Convert Kubernetes Job to Metronome job run format
func JobKubernetesToMetronome(job *batchv1.Job) *MetronomeJobRun {
	ret := &MetronomeJobRun{}
	ret.Id = fmt.Sprintf("%s.%s", job.Namespace, job.Name)
	ret.JobId = fmt.Sprintf("%s.%s", job.Namespace, job.ObjectMeta.OwnerReferences[0].Name)
	ret.CreatedAt = job.CreationTimestamp.String()
	if job.Status.StartTime == nil {
		ret.Status = "STARTING"
	} else if job.Status.Failed > 0 || anyJobConditionsFailed(job.Status.Conditions) {
		ret.Status = "FAILED"
	} else if job.Status.CompletionTime == nil {
		ret.Status = "RUNNING"
	} else {
		ret.Status = "COMPLETED"
		// We need a temp var to be able to use the address of it in the assignment below
		completionTime := job.Status.CompletionTime.String()
		ret.CompletedAt = &completionTime
	}
	ret.Tasks = make([]string, 0)
	return ret
}

// GetMaxTime returns larger from given times, first is time
// second is string, given layout: 2006-01-02 15:04:05.999999999 -0700 MST
func GetMaxTime(time1 time.Time, time2 string) (*time.Time, error) {
	dateTimeLayout := "2006-01-02 15:04:05.999999999 -0700 MST"
	parsedTime2, err := time.Parse(dateTimeLayout, time2)
	if err != nil {
		return nil, err
	}
	if time1.After(parsedTime2) {
		return &time1, nil
	}
	return &parsedTime2, nil
}

// HandleGetJobEmbed handles embed parameter in the query
func HandleGetJobEmbed(embed string, metronomeJob *MetronomeJob) (*MetronomeJob, *GinErrorMessage) {
	namespace, name, err := SplitMetronomeJobId(metronomeJob.Id)
	if err != nil {
		return nil, &GinErrorMessage{HTTPCode: 500, Message: err.Error()}
	}
	switch embed {
	case "history":
		jobs, err := kube.GetJobsFromCronJob(namespace, name)
		if err != nil {
			return nil, &GinErrorMessage{HTTPCode: 500, Message: err.Error()}
		}
		pods, err := kube.GetPods(namespace, "Job")
		if err != nil {
			return nil, &GinErrorMessage{HTTPCode: 500, Message: err.Error()}
		}
		metronomeJob, err = AppendHistoryToMetronomeFromKubeJobs(metronomeJob, jobs, pods)
		if err != nil {
			return nil, &GinErrorMessage{HTTPCode: 500, Message: err.Error()}
		}
		return metronomeJob, nil
	case "activeRuns":
		return nil, &GinErrorMessage{HTTPCode: 200, Message: "TODO"}
	case "schedules":
		return nil, &GinErrorMessage{HTTPCode: 200, Message: "TODO"}
	case "historySummary":
		return nil, &GinErrorMessage{HTTPCode: 200, Message: "TODO"}
	}
	return nil, &GinErrorMessage{
		HTTPCode: 404,
		Message:  "Unknown embed options. Valid options are: activeRuns, schedules, history, historySummary"}
}

// MatchKubeJobWithPods matches pod with its job
// assuming jobId is formatted namespace.jobid and used in pod OwnerReference
func MatchKubeJobWithPods(jobId string, pods []corev1.Pod) []string {
	result := []string{}
	for _, pod := range pods {
		for _, ownerReference := range pod.GetOwnerReferences() {
			if fmt.Sprintf("%s.%s", pod.Namespace, ownerReference.Name) == jobId {
				result = append(result, fmt.Sprintf("%s.%s", pod.Namespace, pod.Name))
				break
			}
		}
	}
	return result
}

// AppendHistoryToMetronomeFromKubeJobs appends job run history to existing MetronomeJob
func AppendHistoryToMetronomeFromKubeJobs(metronomeJob *MetronomeJob, kubeJobs []batchv1.Job, pods []corev1.Pod) (*MetronomeJob, error) {
	failureCount := 0
	successCount := 0
	lastSuccessAtTime := time.Unix(0, 0)
	lastFailureAtTime := time.Unix(0, 0)
	jobHistory := MetronomeJobHistory{
		SuccessfulFinishedRuns: []MetronomeJobHistoryEntry{},
		FailedFinishedRuns:     []MetronomeJobHistoryEntry{}}

	for _, kubeJob := range kubeJobs {
		metronomeJobRun := JobKubernetesToMetronome(&kubeJob)
		jobHistoryEntry := MetronomeJobHistoryEntry{
			ID:         metronomeJobRun.Id,
			CreatedAt:  metronomeJobRun.CreatedAt,
			FinishedAt: metronomeJobRun.CompletedAt,
			Tasks:      MatchKubeJobWithPods(metronomeJobRun.Id, pods)}

		switch metronomeJobRun.Status {
		case "COMPLETED":
			jobHistory.SuccessfulFinishedRuns = append(jobHistory.SuccessfulFinishedRuns, jobHistoryEntry)
			successCount++
			successAtTime, err := GetMaxTime(lastSuccessAtTime, *jobHistoryEntry.FinishedAt)
			if err != nil {
				return nil, err
			}
			lastSuccessAtTime = *successAtTime
		case "FAILED":
			jobHistory.FailedFinishedRuns = append(jobHistory.FailedFinishedRuns, jobHistoryEntry)
			failureCount++
			if anyJobConditionsFailed(kubeJob.Status.Conditions) {
				failureAtTime, err := GetMaxTime(lastFailureAtTime, firstJobConditionFailedTime(kubeJob.Status.Conditions))
				if err != nil {
					return nil, err
				}
				lastFailureAtTime = *failureAtTime
			}
		}
	}

	jobHistory.SuccessCount = successCount
	jobHistory.FailureCount = failureCount
	if successCount > 0 {
		lastSuccessAtString := lastSuccessAtTime.String()
		jobHistory.LastSuccessAt = &lastSuccessAtString
	}
	if failureCount > 0 {
		lastFailureAtString := lastFailureAtTime.String()
		jobHistory.LastFailureAt = &lastFailureAtString
	}
	metronomeJob.History = &jobHistory
	return metronomeJob, nil
}

// Split Metronome job ID into Kubernetes namespace and job name
func SplitMetronomeJobId(jobId string) (string, string, error) {
	parts := strings.SplitN(jobId, ".", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("failed parsing job ID '%s': job ID should consist of namespace and job name separated by a period (namespace.job_name)", jobId)
	}
	return parts[0], parts[1], nil
}
