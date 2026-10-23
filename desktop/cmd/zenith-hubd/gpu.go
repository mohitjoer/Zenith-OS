package main

// gpu.go: the `zenith-hubd gpu ...` subcommand. The Hub GPU page calls these to
// read the hardware capability verdict, switch graphics mode and tune the card.
//
// mux, apply, hook and vm are the GPU-passthrough stack and the ASUS MUX helper.
// Both need tooling Zenith does not ship (Looking Glass, a boot-config writer,
// the ryoku-gpu-mux helper), so they answer "unavailable" instead of erroring.

import (
	"fmt"
	"os/exec"
)

func runGpu(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("gpu needs a subcommand: caps|mode|tune")
	}
	// detection, the render-mode writer and the tuner all go through zenith-gpu
	if _, err := exec.LookPath(zenithGpuBin()); err != nil && args[0] != "mux" && args[0] != "apply" && args[0] != "hook" && args[0] != "vm" {
		return unavailable("gpu "+args[0], zenithGpuBin()+" is not installed", args[1:])
	}
	switch args[0] {
	case "caps":
		report, err := detectCapability()
		if err != nil {
			return err
		}
		return printJSON(report)
	case "mode":
		return runGpuMode(args[1:])
	case "tune":
		return runGpuTune(args[1:])
	case "mux", "apply", "hook", "vm":
		return unavailable("gpu "+args[0], "GPU passthrough and the MUX switch are not set up by Zenith yet", args[1:])
	default:
		return fmt.Errorf("unknown gpu subcommand: %s", args[0])
	}
}
