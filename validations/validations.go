package validations

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func isISO(path string) bool {
	fmt.Println("Validate is a ISO")
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 5)

	// offset 0x8001 onde costuma ter "CD001"
	_, err = f.ReadAt(buf, 0x8001)
	if err != nil {
		return false
	}

	return string(buf) == "CD001"
}

func isBootableISO(path string) bool {
	fmt.Println("Validate is bootable iso")
	cmd := exec.Command("file", path)
	out, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.Contains(string(out), "bootable")
}

func ValidationFiles(filePath string) error {
	fmt.Println("Return success validade")
	if !strings.HasSuffix(strings.ToLower(filePath), ".iso") {
		return errors.New("is not a ISO file")
	}
	if !isBootableISO(filePath) {
		return errors.New("is not a bootale ISO")
	}
	if !isISO(filePath) {
		return errors.New("is not a acceptable ISO")
	}
	return nil
}
