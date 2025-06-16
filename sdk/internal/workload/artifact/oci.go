package artifact

// import (
// 	"fmt"
// 	"os/exec"
// )

// // OCIDefaultHandler handles OCI registry operations.
// type OCIDefaultHandler struct{}

// func (o *OCIDefaultHandler) Pull(source string, dest string, opts ...Option) error {
// 	cmd := exec.Command("oras", "pull", source, "-o", dest)
// 	output, err := cmd.CombinedOutput()
// 	if err != nil {
// 		return fmt.Errorf("OCI pull failed: %v, output: %s", err, string(output))
// 	}
// 	fmt.Println("Pulled OCI artifact:", source)
// 	return nil
// }

// func (o *OCIDefaultHandler) Push(source string, dest string, opts ...Option) error {
// 	cmd := exec.Command("oras", "push", dest, source)
// 	output, err := cmd.CombinedOutput()
// 	if err != nil {
// 		return fmt.Errorf("OCI push failed: %v, output: %s", err, string(output))
// 	}
// 	fmt.Println("Pushed OCI artifact:", source, "to", dest)
// 	return nil
// }

// func (o *OCIDefaultHandler) Verify(source string, opts ...Option) error {
// 	// TODO: Implement OCI artifact verification
// 	fmt.Println("Verified OCI artifact:", source)
// 	return nil
// }
