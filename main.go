package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/FitrahHaque/Compression-Engine/engine"
)

var Commands = [...]string{"compress", "decompress", "benchmark", "help", "server"}

func main() {
	application := os.Args[0]
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	compressCmd := flag.Bool(Commands[0], false, "Compress File")
	decompressCmd := flag.Bool(Commands[1], false, "Decompress File")
	benchmarkCmd := flag.Bool(Commands[2], false, "Benchmark File")
	serverCmd := flag.Bool(Commands[4], false, "Create a server")
	helpCmd := flag.Bool(Commands[3], false, "Help")

	if len(os.Args) == 1 {
		fmt.Println("Please provide commands")
		os.Exit(1)
	}
	commandArgs := findIntersection(
		[]string{
			"--server",
			"--compress",
			"--decompress",
			"--benchmark",
		},
		os.Args[1:2],
	)
	flag.CommandLine.Parse(commandArgs)
	commandsSelected := countTrue([]bool{*compressCmd, *decompressCmd, *benchmarkCmd, *serverCmd})
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
	checkForCompress(application, "", compressCmd, 1)
	checkForDecompress(application, "", decompressCmd, 1)
	checkForServer(application, "", serverCmd, 1)
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

func checkForAlgorithm(application, prefix string, algorithmChosen *string, algorithmIdx int) any {
	var args any
	if *algorithmChosen == "flate" {
		// fmt.Printf("[ main ] flate algorithm chosen\n")
		flateCompressFS := flag.NewFlagSet("flate", flag.ExitOnError)
		flateCompressFS.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage of %s %s --algorithm=flate [OPTIONS] <file(s)>\n", application, prefix)
			fmt.Fprintf(os.Stderr, "Valid commands include:\n\t%s\n", strings.Join([]string{"btype, bfinal, help"}, ", "))
			fmt.Fprintf(os.Stderr, "Flag:\n")
			flateCompressFS.PrintDefaults()
		}
		btypeFlateCompress := flateCompressFS.Int("btype", 2, "Which btype to use, choices include: 1, 2, 3")
		bfinalFlateCompress := flateCompressFS.Int("bfinal", 0, "Final Block of the compression process")
		helpFlateCompress := flateCompressFS.Bool("help", false, "Compress Help")
		commandArgs := findIntersection(
			[]string{
				"--btype",
				"--bfinal",
			},
			os.Args[algorithmIdx+2:],
		)
		// fmt.Println(commandArgs)
		if len(commandArgs) == 0 {
			commandArgs = findIntersection(
				[]string{
					"--help",
				},
				os.Args[algorithmIdx+2:],
			)
		}
		flateCompressFS.Parse(commandArgs)
		if *helpFlateCompress {
			flateCompressFS.Usage()
		}
		args = engine.FlateArgs{
			Btype:  uint32(*btypeFlateCompress),
			Bfinal: uint32(*bfinalFlateCompress),
		}
	}
	if *algorithmChosen == "gzip" {
		// fmt.Printf("[ main ] flate algorithm chosen\n")
		gzipCompressFS := flag.NewFlagSet("gzip", flag.ExitOnError)
		gzipCompressFS.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage of %s --compress --algorithm=gzip [OPTIONS] <file(s)>\n", application)
			fmt.Fprintf(os.Stderr, "Valid commands include:\n\t%s\n", strings.Join([]string{"btype, bfinal, help"}, ", "))
			fmt.Fprintf(os.Stderr, "Flag:\n")
			gzipCompressFS.PrintDefaults()
		}
		btypeGzipCompress := gzipCompressFS.Int("btype", 2, "Which btype to use, choices include: 1, 2, 3")
		bfinalGzipCompress := gzipCompressFS.Int("bfinal", 0, "Final Block of the compression process")
		helpGzipCompress := gzipCompressFS.Bool("help", false, "Compress Help")
		commandArgs := findIntersection(
			[]string{
				"--btype",
				"--bfinal",
			},
			os.Args[algorithmIdx+2:],
		)
		// fmt.Println(commandArgs)
		if len(commandArgs) == 0 {
			commandArgs = findIntersection(
				[]string{
					"--help",
				},
				os.Args[algorithmIdx+2:],
			)
		}
		gzipCompressFS.Parse(commandArgs)
		if *helpGzipCompress {
			gzipCompressFS.Usage()
		}
		args = engine.GzipArgs{
			Btype:  uint32(*btypeGzipCompress),
			Bfinal: uint32(*bfinalGzipCompress),
		}
	}
	return args
}

func checkForFiles(startIdx int) []string {
	var fileName string
	if len(os.Args) > startIdx {
		i := startIdx + 1
		for ; i < len(os.Args) && os.Args[i][0] == '-'; i++ {
		}
		if i == len(os.Args) {
			fmt.Println("No file provided for content encoding")
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
	return files
}

func checkForCompress(application string, prefix string, compressCmd *bool, compressIdx int) {
	if *compressCmd {
		compressFS := flag.NewFlagSet("compress", flag.ExitOnError)
		compressFS.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage of %s --compress [OPTIONS] <file(s)>\n", application)
			fmt.Fprintf(os.Stderr, "Valid commands include:\n\t%s\n", strings.Join([]string{"algorithm, delete, outfileext, help"}, ", "))
			fmt.Fprintf(os.Stderr, "Flag:\n")
			compressFS.PrintDefaults()
		}
		algorithmCompress := compressFS.String("algorithm", "huffman", fmt.Sprintf("Which algorithm(s) to use, choices include: \n\t%s", strings.Join(engine.Engines[:], ", ")))
		deleteAfterCompress := compressFS.Bool("delete", false, "Delete file after compression")
		outputFileExtensionCompress := compressFS.String("outfileext", ".shk", "File extension used for the result")
		helpCompress := compressFS.Bool("help", false, "Compress Help")
		commandArgs := findIntersection(
			[]string{
				"--algorithm",
				"--delete",
				"--outfileext",
			},
			os.Args[compressIdx+1:],
		)
		// fmt.Println(commandArgs)
		if len(commandArgs) == 0 {
			commandArgs = findIntersection(
				[]string{
					"--help",
				},
				os.Args[compressIdx+1:],
			)
		}
		compressFS.Parse(commandArgs)
		if *helpCompress {
			compressFS.Usage()
		}

		files := checkForFiles(compressIdx)
		// algorithmsChosen := strings.Split(*algorithmCompress, ",")
		// trimSpace(algorithmsChosen)
		// engine.CompressFiles(algorithmsChosen, files, *outputFileExtensionCompress)
		subPrefix := strings.Join([]string{prefix, fmt.Sprintf("--%s", "compress")}, " ")
		args := checkForAlgorithm(application, subPrefix, algorithmCompress, compressIdx+1)

		engine.CompressFiles(*algorithmCompress, files, *outputFileExtensionCompress, args)
		if *deleteAfterCompress {
			deleteFiles(files)
		}
	}
}

func checkForDecompress(application string, prefix string, decompressCmd *bool, decompressIdx int) {
	if *decompressCmd {
		// fmt.Printf("os.Args[2:]: %v\n", os.Args[2:])
		decompressFS := flag.NewFlagSet("decompress", flag.ExitOnError)
		decompressFS.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage of %s %s --decompress [OPTIONS] <file(s)>\n", application, prefix)
			fmt.Fprintf(os.Stderr, "Valid commands include:\n\t%s\n", strings.Join([]string{"algorithm, delete, help"}, ", "))
			fmt.Fprintf(os.Stderr, "Flag:\n")
			decompressFS.PrintDefaults()
		}
		deleteAfterDecompress := decompressFS.Bool("delete", false, "Delete compression file after decompression")
		algorithmDecompress := decompressFS.String("algorithm", "huffman", fmt.Sprintf("Which algorithm(s) to use, choices include: \n\t%s", strings.Join(engine.Engines[:], ", ")))
		helpDecompress := decompressFS.Bool("help", false, "Help")
		commandArgs := findIntersection(
			[]string{
				"--algorithm",
				"--delete",
				"--help",
			},
			os.Args[decompressIdx+1:],
		)
		// fmt.Printf("len(commandArgs): %v\n", len(commandArgs))
		if len(commandArgs) == 0 {
			commandArgs = findIntersection(
				[]string{
					"--help",
				},
				os.Args[decompressIdx+1:],
			)
		}
		decompressFS.Parse(commandArgs)
		if *helpDecompress {
			decompressFS.Usage()
		}
		files := checkForFiles(decompressIdx)
		// algorithmsChosen := strings.Split(*algorithmDecompress, ",")
		// trimSpace(algorithmsChosen)
		// engine.DecompressFiles(algorithmsChosen, files)
		engine.DecompressFiles(*algorithmDecompress, files)
		if *deleteAfterDecompress {
			deleteFiles(files)
		}
	}
}

func checkForServer(application string, prefix string, serverCmd *bool, serverIdx int) {
	if *serverCmd {
		serverFS := flag.NewFlagSet("server", flag.ExitOnError)
		// compressCmd := flag.Bool("compress", false, "Compress File")
		// decompressCmd := flag.Bool("decompress", false, "Decompress File")
		// compressionPortCmd := flag.Int("Compression Port", 8080, "Compression Data Port")
		serverPortCmd := serverFS.Int("serverPort", 8080, "Decompression Server Port")
		algorithmCmd := serverFS.String("algorithm", "gzip", fmt.Sprintf("Which algorithm(s) to use, choices include: \n\t%s", strings.Join(engine.Engines[:], ", ")))
		helpCmd := serverFS.Bool("help", false, "Help")
		serverFS.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage of %s %s --server [OPTIONS] <file(s)>\n", application, prefix)
			fmt.Fprintf(os.Stderr, "Valid commands include:\n\t%s\n", strings.Join([]string{"serverPort, algorithm, help"}, ", "))
			fmt.Fprintf(os.Stderr, "Flag:\n")
			serverFS.PrintDefaults()
		}
		commandArgs := findIntersection(
			[]string{
				"--serverPort",
				"--algorithm",
				"--help",
			},
			os.Args[serverIdx+1:],
		)
		// fmt.Printf("len(commandArgs): %v\n", len(commandArgs))
		if len(commandArgs) == 0 {
			commandArgs = findIntersection(
				[]string{
					"--help",
				},
				os.Args[serverIdx+1:],
			)
		}
		serverFS.Parse(commandArgs)
		if *helpCmd {
			serverFS.Usage()
		}
		// files := checkForFiles(serverIdx)
		subPrefix := strings.Join([]string{prefix, fmt.Sprintf("--%s", "server")}, " ")
		args := checkForAlgorithm(application, subPrefix, algorithmCmd, serverIdx+1)
		files := checkForFiles(serverIdx)
		// server
		go func() {
			fmt.Printf("Server starting at port %v...\n", *serverPortCmd)
			if err := http.ListenAndServe(fmt.Sprintf(":%v", *serverPortCmd), compressionMiddleware(http.HandlerFunc(dataHandler))); err != nil {
				log.Fatal(err)
			}
		}()

		// client
		req, _ := http.NewRequest("POST", fmt.Sprintf("http://localhost:%v/", *serverPortCmd), engine.ClientCompress(*algorithmCmd, files[0], args))
		req.Header.Set("Content-Encoding", *algorithmCmd)
		res, err := http.DefaultClient.Do(req)
		if err != nil || res.StatusCode != 200 {
			log.Fatal(err, res)
		}
	}
}

func findIntersection(commandList, argList []string) []string {
	set := make(map[string]struct{}, len(commandList))
	for _, c := range commandList {
		set[c] = struct{}{}
	}
	var out []string
	for _, arg := range argList {
		cmd := arg
		if strings.Contains(cmd, "=") {
			cmd = strings.SplitN(cmd, "=", 2)[0]
		}
		if _, ok := set[cmd]; ok {
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

// type Codec interface {
// 	NewWriter(io.Writer) io.WriteCloser
// 	NewReader(io.Reader) io.ReadCloser
// }

func compressionMiddleware(handler http.Handler) http.Handler {
	// fmt.Printf("[ compressionMiddleware ]\n")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if value := r.Header.Get("Content-Encoding"); value == "" {
			handler.ServeHTTP(w, r)
			return
		} else {
			r.Body = engine.ServerDecompress(value, r.Body)
			handler.ServeHTTP(w, r)
		}
	})
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	// fmt.Printf("[ dataHandler ]\n")
	if data, err := io.ReadAll(r.Body); err != nil {
		panic(err)
	} else {
		os.WriteFile("server-decompressed.txt", data, 0644)
		fmt.Println("Client Data has been saved into `server-decompressed.txt`")
	}
}
