package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/FitrahHaque/Compression-Engine/engine"
)

var Commands = [...]string{"compress", "decompress", "benchmark", "help"}

func main() {
	application := os.Args[0]
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	compressCmd := flag.Bool(Commands[0], false, "Compress File")
	decompressCmd := flag.Bool(Commands[1], false, "Decompress File")
	benchmarkCmd := flag.Bool(Commands[2], false, "Benchmark File")
	helpCmd := flag.Bool(Commands[3], false, "Help")

	if len(os.Args) == 1 {
		fmt.Println("Please provide commands")
		os.Exit(1)
	}
	commandArgs := findIntersection(
		[]string{
			"--compress",
			"--decompress",
			"--benchmark",
		},
		os.Args[1:],
	)
	flag.CommandLine.Parse(commandArgs)
	commandsSelected := countTrue([]bool{*compressCmd, *decompressCmd, *benchmarkCmd})
	if commandsSelected > 1 {
		fmt.Println("Specify a single command")
		os.Exit(1)
	} else if commandsSelected == 0 {
		commandArgs = findIntersection(
			[]string{
				"--help",
			},
			os.Args[1:],
		)
		flag.CommandLine.Parse(commandArgs)
		if *helpCmd {
			fmt.Fprintf(os.Stderr, "Usage of %s:\n", application)
			fmt.Fprintf(os.Stderr, "Valid commands include:\n\t%s\n", strings.Join(Commands[:], ", "))
			fmt.Fprintf(os.Stderr, "Flag:\n")
			flag.PrintDefaults()
			return
		}
		fmt.Println("No command is selected. Compression by default")
		cmdTrue := true
		compressCmd = &cmdTrue
	}

	if *compressCmd {
		compressFS := flag.NewFlagSet("compress", flag.ExitOnError)
		compressFS.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage of %s --compress [OPTIONS] <file(s)>\n", application)
			fmt.Fprintf(os.Stderr, "Valid commands include:\n\t%s\n", strings.Join([]string{"algorithm, delete, outfileext, help"}, ", "))
			fmt.Fprintf(os.Stderr, "Flag:\n")
			compressFS.PrintDefaults()
			return
		}
		algorithmCompress := compressFS.String("algorithm", "huffman", fmt.Sprintf("Which algorithm(s) to use, choices include: \n\t%s", strings.Join(engine.Engines[:], ", ")))
		deleteAfterCompress := compressFS.Bool("delete", false, "Delete file after compression")
		helpCompress := compressFS.Bool("help", false, "Compress Help")
		commandArgs = findIntersection(
			[]string{
				"--algorithm",
				"--delete",
				"--outfileext",
			},
			os.Args[2:],
		)
		if len(commandArgs) == 0 {
			commandArgs = findIntersection(
				[]string{
					"--help",
				},
				os.Args[2:],
			)
		}
		compressFS.Parse(commandArgs)
		if *helpCompress {
			compressFS.Usage()
		}

		var fileName string
		if len(os.Args) > 1 {
			i := 1
			for ; i < len(os.Args) && os.Args[i][0] == '-'; i++ {
			}
			if i == len(os.Args) {
				fmt.Println("No file provided for compression")
				os.Exit(1)
			}
			fileName = os.Args[i]
		}
		if strings.Contains(fileName, ",") {
			for _, f := range strings.Split(fileName, ",") {
				if _, err := os.Stat(f); os.IsNotExist(err) {
					fmt.Printf("Could not open the provided file %s\n", f)
					os.Exit(1)
				}
			}
		} else if _, err := os.Stat(fileName); os.IsNotExist(err) {
			fmt.Printf("Could not open the provided file %s\n", fileName)
			os.Exit(1)
		}
		files := strings.Split(fileName, ",")
		trimSpace(files)
		outputFileExtensionCompress := compressFS.String("outfileext", "rsn", "File extension used for the result")
		algorithmsChosen := strings.Split(*algorithmCompress, ",")
		trimSpace(algorithmsChosen)
		engine.CompressFiles(algorithmsChosen, files, *outputFileExtensionCompress)
		if *deleteAfterCompress {
			deleteFiles(files)
		}
	}
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

func findIntersection(commandList, argList []string) []string {
	set := make(map[string]struct{}, len(commandList))
	for _, c := range commandList {
		set[c] = struct{}{}
	}
	var out []string
	for _, arg := range argList {
		if _, ok := set[arg]; ok {
			out = append(out, arg)
		}
	}
	return out
}

func trimSpace(s []string) {
	for i := range s {
		s[i] = strings.TrimSpace(s[i])
	}
}

func deleteFiles(files []string) {
	for _, file := range files {
		if err := os.Remove(file); err != nil {
			panic(err)
		}
	}
}
