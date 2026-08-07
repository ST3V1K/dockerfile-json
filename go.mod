module github.com/keilerkonzept/dockerfile-json

go 1.26.3

require (
	github.com/containerd/platforms v1.0.0-rc.4 // for ARM variant detection
	github.com/google/go-cmp v0.7.0
	github.com/moby/buildkit v0.32.2
	github.com/yalp/jsonpath v0.0.0-20180802001716-5cc68e5049a0
)

require (
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/typeurl/v2 v2.3.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/tonistiigi/go-csvvalue v0.0.0-20240814133006-030d3b2625d0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	google.golang.org/protobuf v1.36.12-0.20260120151049-f2248ac996af // indirect
)

replace (
	github.com/containerd/containerd => github.com/containerd/containerd v1.7.22
	github.com/docker/docker/v23/v20 => github.com/docker/docker v27.3.1+incompatible
)
