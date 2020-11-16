package hook

import "testing"

func TestSplitDockerDomain(t *testing.T) {
	tests := []struct {
		name              string
		image             string
		expectedDomain    string
		expectedRemainder string
	}{
		{
			name:              "docker hub domain",
			image:             "docker.io/nginx:latest",
			expectedDomain:    "docker.io",
			expectedRemainder: "library/nginx:latest",
		},
		{
			name:              "docker hub domain omitting image tag",
			image:             "nginx",
			expectedDomain:    "docker.io",
			expectedRemainder: "library/nginx",
		},
		{
			name:              "docker hub domain omitting domain",
			image:             "nginx",
			expectedDomain:    "docker.io",
			expectedRemainder: "library/nginx",
		},
		{
			name:              "ecr domain",
			image:             "123456789.dkr.ecr.ap-northeast-1.amazonaws.com/nginx:latest",
			expectedDomain:    "123456789.dkr.ecr.ap-northeast-1.amazonaws.com",
			expectedRemainder: "nginx:latest",
		},
		{
			name:              "gcr domain",
			image:             "k8s.gcr.io/autoscaling/cluster-autoscaler:v1.17.4",
			expectedDomain:    "k8s.gcr.io",
			expectedRemainder: "autoscaling/cluster-autoscaler:v1.17.4",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			domain, remainder := splitDockerDomain(tt.image)

			if domain != tt.expectedDomain {
				t.Errorf("not expected condition. actual: %s, expected: %s", domain, tt.expectedDomain)
			}

			if remainder != tt.expectedRemainder {
				t.Errorf("not expected condition. actual: %s, expected: %s", remainder, tt.expectedRemainder)
			}
		})
	}
}
