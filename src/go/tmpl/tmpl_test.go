package tmpl_test

import (
	"bytes"
	"strings"
	"testing"

	"phenix/tmpl"
)

type windowsTemplateData struct {
	Node     windowsTemplateNode
	Metadata map[string]any
}

type windowsTemplateNode struct {
	Annotations map[string]any
	Network     windowsTemplateNetwork
	General     windowsTemplateGeneral
}

type windowsTemplateNetwork struct {
	Interfaces []windowsTemplateInterface
	Routes     []windowsTemplateRoute
}

type windowsTemplateInterface struct {
	Proto       string
	QinQ        bool
	Address     string
	NetworkMask string
	Gateway     string
	DNS         []string
}

type windowsTemplateRoute struct {
	Next        string
	Destination string
}

type windowsTemplateGeneral struct {
	Hostname string
}

func TestWindowsStartupTemplateUsesLeastPrivilegeMinicccAccess(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		metadata         map[string]any
		expectedIdentity string
	}{
		{
			name: "standalone local autologon user",
			metadata: map[string]any{
				"auto_logon": map[string]any{
					"username": "alice",
					"password": "pw",
				},
			},
			expectedIdentity: `$autoLogonIdentity = ".\alice"`,
		},
		{
			name: "domain joined local autologon user",
			metadata: map[string]any{
				"domain_controller": map[string]any{
					"ip":       "10.0.0.10",
					"domain":   "example.test",
					"username": "administrator@example.test",
					"password": "pw",
				},
				"auto_logon": map[string]any{
					"username": "alice",
					"password": "pw",
					"local":    true,
				},
			},
			expectedIdentity: `$autoLogonIdentity = ".\alice"`,
		},
		{
			name: "domain joined domain autologon user",
			metadata: map[string]any{
				"domain_controller": map[string]any{
					"ip":       "10.0.0.10",
					"domain":   "example.test",
					"username": "administrator@example.test",
					"password": "pw",
				},
				"auto_logon": map[string]any{
					"username": "alice",
					"password": "pw",
					"local":    false,
				},
			},
			expectedIdentity: `$autoLogonIdentity = "$domain\alice"`,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer
			data := windowsTemplateData{
				Node: windowsTemplateNode{
					Annotations: map[string]any{},
					Network:     windowsTemplateNetwork{},
					General: windowsTemplateGeneral{
						Hostname: "win-node",
					},
				},
				Metadata: tc.metadata,
			}

			if err := tmpl.GenerateFromTemplate("windows_startup.tmpl", data, &output); err != nil {
				t.Fatalf("rendering windows startup template: %v", err)
			}

			rendered := output.String()
			if strings.Contains(rendered, `Add-LocalGroupMember -Group "Administrators"`) {
				t.Fatalf("rendered script still contains Administrators workaround")
			}

			if !strings.Contains(rendered, "function Phenix-EnsureMinicccAccess($autoLogonIdentity)") {
				t.Fatalf("rendered script missing miniccc access helper function")
			}

			if !strings.Contains(rendered, tc.expectedIdentity) {
				t.Fatalf("rendered script missing expected auto-logon identity assignment %q", tc.expectedIdentity)
			}

			if !strings.Contains(rendered, "Phenix-EnsureMinicccAccess $autoLogonIdentity") {
				t.Fatalf("rendered script missing miniccc access setup call")
			}

			if !strings.Contains(rendered, "sandia-minimega/minimega#1560") {
				t.Fatalf("rendered script missing minimega issue reference comment")
			}
		})
	}
}
