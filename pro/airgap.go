//go:build ee && airgap
// +build ee,airgap

package pro

func isAirgapped() bool {
	return true
}
