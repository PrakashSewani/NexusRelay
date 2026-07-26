package config

import (
	"strings"
	"testing"
)

func TestEnvironmentFileStrictParsing(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "unknown", input: "DATABSE_HOST=postgres\n", want: "unknown key DATABSE_HOST"},
		{name: "duplicate", input: "DATABASE_HOST=one\nDATABASE_HOST=two\n", want: "duplicate key DATABASE_HOST"},
		{name: "invalid name", input: "database_host=postgres\n", want: "invalid key"},
		{name: "export", input: "export DATABASE_HOST=postgres\n", want: "export syntax"},
		{name: "malformed", input: "DATABASE_HOST\n", want: "expected KEY=VALUE"},
		{name: "nul", input: "DATABASE_HOST=post\x00gres\n", want: "control character"},
		{name: "control", input: "DATABASE_HOST=post\tgres\n", want: "control character"},
		{name: "invalid utf8", input: string([]byte{'D', 'A', 'T', 'A', 'B', 'A', 'S', 'E', '_', 'H', 'O', 'S', 'T', '=', 0xff, '\n'}), want: "valid UTF-8"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseEnvironmentFile(strings.NewReader(test.input))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if strings.Contains(err.Error(), "postgres") {
				t.Fatalf("error disclosed value: %v", err)
			}
		})
	}
}

func TestEnvironmentFileAllowsEmptyValuesWithoutInterpolation(t *testing.T) {
	values, err := parseEnvironmentFile(strings.NewReader("OTEL_EXPORTER_OTLP_ENDPOINT=\nADMIN_ORIGINS=${ADMIN_BASE_URL}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if values["OTEL_EXPORTER_OTLP_ENDPOINT"] != "" || values["ADMIN_ORIGINS"] != "${ADMIN_BASE_URL}" {
		t.Fatalf("values = %#v", values)
	}
}

func TestAmbientTypoIsIgnoredButEnvFileRejectsIt(t *testing.T) {
	values := fixture(t)
	values["DATABSE_HOST"] = "postgres"
	if _, err := ParseGateway(values); err != nil {
		t.Fatalf("ambient typo was claimed: %v", err)
	}
	if _, err := parseEnvironmentFile(strings.NewReader("DATABSE_HOST=postgres\n")); err == nil {
		t.Fatal("env-file typo accepted")
	}
}

func TestValidateCompleteChecksDeploymentBeforeProcessViews(t *testing.T) {
	values := fixture(t)
	values["DATABASE_GATEWAY_USER"] = gatewayUser
	values["DATABASE_CONTROL_PLANE_USER"] = "wrong-control-plane-user"
	if err := ValidateComplete(values); err == nil || !strings.Contains(err.Error(), "DATABASE_CONTROL_PLANE_USER") {
		t.Fatalf("deployment principal error = %v", err)
	}

	values = fixture(t)
	values["DATABASE_GATEWAY_USER"] = "wrong-gateway-user"
	values["DATABASE_CONTROL_PLANE_USER"] = controlPlaneUser
	if _, err := ParseControlPlane(ValuesForProcess(values, ProcessControlPlane)); err != nil {
		t.Fatalf("control-plane process view consumed gateway principal: %v", err)
	}
	if err := ValidateComplete(values); err == nil || !strings.Contains(err.Error(), "DATABASE_GATEWAY_USER") {
		t.Fatalf("full validator did not reject deployment principal: %v", err)
	}
}
