package px_init

import (
	"bytes"
	"fmt"
	"os/exec"
)

func enableSharedMounts() error {
	cmd := exec.Command("nsenter", "--mount=/media/host/proc/1/ns/mnt", "--", "mount", "--make-shared", "/")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		fmt.Printf("Failed to enable shared mounts. Err: %v Stderr: %v\n", err, stderr)
		return err
	}

	fmt.Println("Enabled shared mounts successfully")
	return nil
}

func main() {
	enableSharedMounts()
}
