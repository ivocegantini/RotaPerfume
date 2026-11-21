package deploy_test

// DEPLOY-09: o setup-linux.sh instala o docker-buildx (sem ele o "docker
// build" cai no legacy builder) e avisa quando o plugin falta.

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDEPLOY09_SetupLinuxInstalaBuildx(t *testing.T) {
	sh := lerRaiz(t, "scripts", "setup-linux.sh")
	assert.Contains(t, sh, "apt_instalar docker-buildx")
	assert.Contains(t, sh, "\tdocker_buildx_instalar\n", "chamado na etapa_docker")
	assert.Contains(t, sh, "sem o plugin 'docker buildx'")
}
