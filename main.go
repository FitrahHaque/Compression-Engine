package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var Commands = [...]string{"compress", "decompress", "benchmark", "help"}

func main() {
	application := os.Args[0]
	fmt.Println(os.Args[0])
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	compressCmd := flag.Bool(Commands[0], false, "Compress File")
	decompressCmd := flag.Bool(Commands[1], false, "Decompress File")
	benchmarkCmd := flag.Bool(Commands[2], false, "Benchmark File")
	helpCmd := flag.Bool(Commands[3], false, "Help")

	commandArgs := make([]string, len(os.Args[:]))
	copy(commandArgs, os.Args[:])
	if len(commandArgs) > 1 {
		commandArgs = commandArgs[1:2]
	}
	fmt.Println(commandArgs[0])
	if commandArgs[0] == "-compress" || commandArgs[0] == "-decompress" || commandArgs[0] == "-benchmark" || commandArgs[0] == "-help" {
		flag.CommandLine.Parse(commandArgs)
	}
	commandsSelected := countTrue([]bool{*compressCmd, *decompressCmd, *benchmarkCmd, *helpCmd})
	if commandsSelected > 1 {
		fmt.Println("Specify a single command")
		os.Exit(1)
	} else if commandsSelected == 0 {
		fmt.Println("No command is selected. Compression by default")
		cmdTrue := true
		compressCmd = &cmdTrue
	}
	fmt.Println("All ok")
	
	if *helpCmd {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", application)
		fmt.Fprintf(os.Stderr, "Valid commands include:\n\t%s\n", strings.Join(Commands[:], ", "))
		fmt.Fprintf(os.Stderr, "Flag:\n")
		flag.PrintDefaults()
		return
	}
	return
	// var generateHTML *bool
	// if *benchmarkCmd {
	// 	generateHTML = flag.Bool("generate", false, "Compile benchmark results as an html file")
	// }
}

func countTrue(commands []bool) int {
	count := 0
	for _, c := range commands {
		if c == true {
			count++
		}
	}
	return count
}