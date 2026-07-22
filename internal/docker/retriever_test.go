package docker

import (
	"testing"
)

// inspectFixture is real `docker inspect <id>` output captured from a running
// container (see internal/docker/retriever_test.go comment below for
// provenance), with NetworkSettings.Ports and a second Networks entry added
// by hand to exercise the port-binding and multi-network-sort paths that the
// sampled container (which had no published ports and only one network)
// didn't cover.
const inspectFixture = `[
    {
        "Id": "51d9e463209e4e817d650b83ca7b6fb14e13bae27224244d773721a88bc3d216",
        "Created": "2026-07-04T14:40:17.086539796Z",
        "State": {
            "Status": "running",
            "StartedAt": "2026-07-09T18:18:26.961948833Z",
            "FinishedAt": "0001-01-01T00:00:00Z"
        },
        "Image": "sha256:9de60a6b3cad98afa76f2f7c8c452555572ef7992960bdcce014949175251223",
        "Name": "/skillui",
        "HostConfig": {
            "RestartPolicy": {
                "Name": "unless-stopped",
                "MaximumRetryCount": 0
            }
        },
        "Mounts": [
            {
                "Type": "bind",
                "Source": "/Users/colindickson/.claude",
                "Destination": "/data/claude",
                "Mode": "ro",
                "RW": false,
                "Propagation": "rprivate"
            }
        ],
        "Config": {
            "Cmd": null,
            "Image": "skillui-skillui",
            "Labels": {
                "com.docker.compose.project": "skillui",
                "com.docker.compose.service": "skillui"
            }
        },
        "NetworkSettings": {
            "Ports": {
                "8000/tcp": [
                    {
                        "HostIp": "0.0.0.0",
                        "HostPort": "8787"
                    }
                ],
                "443/tcp": null
            },
            "Networks": {
                "zzz_network": {
                    "IPAMConfig": null,
                    "NetworkID": "zzz",
                    "Gateway": "10.0.1.1",
                    "IPAddress": "10.0.1.5",
                    "MacAddress": "02:42:0a:00:01:05"
                },
                "aaa_network": {
                    "IPAMConfig": null,
                    "NetworkID": "aaa",
                    "Gateway": "10.0.0.1",
                    "IPAddress": "10.0.0.2",
                    "MacAddress": "02:42:0a:00:00:02"
                }
            }
        }
    }
]`

func TestParseInspectOutput(t *testing.T) {
	r := &DefaultRetriever{}
	info := &ContainerInfo{}

	if err := r.parseInspectOutput(info, []byte(inspectFixture)); err != nil {
		t.Fatalf("parseInspectOutput returned error: %v", err)
	}

	if info.Name != "skillui" {
		t.Errorf("Name = %q, want %q", info.Name, "skillui")
	}
	if info.Image != "skillui-skillui" {
		t.Errorf("Image = %q, want %q", info.Image, "skillui-skillui")
	}
	if info.Command != "" {
		t.Errorf("Command = %q, want empty (Cmd was null)", info.Command)
	}
	if got, want := info.Labels["com.docker.compose.service"], "skillui"; got != want {
		t.Errorf("Labels[service] = %q, want %q", got, want)
	}
	if info.State != "running" || info.Status != "running" {
		t.Errorf("State/Status = %q/%q, want running/running", info.State, info.Status)
	}
	// CreatedAt must come from the top-level "Created" field, not
	// State.StartedAt (a container created last week and started an
	// hour ago has different values for each).
	if info.CreatedAt != "2026-07-04T14:40:17.086539796Z" {
		t.Errorf("CreatedAt = %q, want the top-level Created timestamp", info.CreatedAt)
	}
	if info.ImageID != "9de60a6b3cad" {
		t.Errorf("ImageID = %q, want %q", info.ImageID, "9de60a6b3cad")
	}
	if info.RestartPolicy != "unless-stopped" {
		t.Errorf("RestartPolicy = %q, want %q", info.RestartPolicy, "unless-stopped")
	}
	if info.Platform != "" {
		t.Errorf("Platform = %q, want empty (not present in fixture)", info.Platform)
	}

	// Port bindings: nil binding list for 443/tcp must be skipped, and the
	// one real binding must decode.
	if len(info.Ports) != 1 {
		t.Fatalf("len(Ports) = %d, want 1", len(info.Ports))
	}
	p := info.Ports[0]
	if p.ContainerPort != "8000" || p.Protocol != "tcp" || p.HostIP != "0.0.0.0" || p.HostPort != "8787" {
		t.Errorf("Ports[0] = %+v, unexpected", p)
	}
	if info.PortString != "8787->8000/tcp" {
		t.Errorf("PortString = %q, want %q", info.PortString, "8787->8000/tcp")
	}

	// Multi-network determinism: alphabetically first network name wins.
	if len(info.Networks) != 2 || info.Networks[0] != "aaa_network" || info.Networks[1] != "zzz_network" {
		t.Errorf("Networks = %v, want sorted [aaa_network zzz_network]", info.Networks)
	}
	if info.IPAddress != "10.0.0.2" || info.Gateway != "10.0.0.1" || info.MacAddress != "02:42:0a:00:00:02" {
		t.Errorf("primary network fields = %q/%q/%q, want aaa_network's values", info.IPAddress, info.Gateway, info.MacAddress)
	}

	// Mounts
	if len(info.Mounts) != 1 {
		t.Fatalf("len(Mounts) = %d, want 1", len(info.Mounts))
	}
	m := info.Mounts[0]
	if m.Type != "bind" || m.Source != "/Users/colindickson/.claude" || m.Destination != "/data/claude" || m.Mode != "ro" {
		t.Errorf("Mounts[0] = %+v, unexpected", m)
	}
}

func TestParseInspectOutput_NoData(t *testing.T) {
	r := &DefaultRetriever{}
	info := &ContainerInfo{}

	if err := r.parseInspectOutput(info, []byte(`[]`)); err == nil {
		t.Error("expected error for empty inspect array, got nil")
	}
}
