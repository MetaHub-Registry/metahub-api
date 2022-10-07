package types

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func TestMain(m *testing.M) {
	logrus.Info("Start TestMain")
	m.Run()
}
