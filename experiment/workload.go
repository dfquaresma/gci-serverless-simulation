package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	repoPath         = "/home/ubuntu/gci-faas-sim/"
	runtimePath      = repoPath + "runtime/thumb-func/"
	noGciEntryPoint  = "entrypoint_port=8080 "
	gciEntryPoint    = "entrypoint_port=8082 "
	scale            = "scale=0.1 "
	runtimeCoreSet   = "taskset 0x1 nice -20 "
	proxyCoreSet     = "taskset 0x2 nice -20 "
	heapSize         = "-Xms512m -Xmx512m " // minimum and maximum heap size of 512mb, from a virtual ambient of 512mb
	proxyYgen        = "--ygen=52428800 "   // proxy forces it's collects after 100mb of heap usage
	awsJvmFlags      = "-XX:MaxMetaspaceSize=52429k -XX:ReservedCodeCacheSize=26214k -Xshare:on -XX:-TieredCompilation -XX:+UseSerialGC -Djava.net.preferIPv4Stack=true "
	noGcijavaGCFlags = "-server " + heapSize + awsJvmFlags
	gcijavaGCFlags   = "-server " + heapSize + awsJvmFlags + "-XX:NewRatio=1 " // ~210mb to new generation, ~210mb to old generation
	proxyFlags       = "--port=8080 --target=127.0.0.1:8082 --gci_target=127.0.0.1:8500 " + proxyYgen
	jarPath          = runtimePath + "target/thumbnailator-server-maven-0.0.1-SNAPSHOT.jar "
	funcName         = "thumb"
)

var (
	expId       = flag.String("expid", "test", "Experiment's ID, default value is test")
	useGci      = flag.Bool("usegci", false, "Whether to use GCI, default false")
	target      = flag.String("target", "", "function's ip and port separated as host:port. There's no default value and should not start with http")
	logPath     = flag.String("logpath", "", "the absolute path to save logs. It has no default value")
	nReqs       = flag.Int64("nreqs", 10000, "number of requests, default 10000")
	resultsPath = flag.String("resultspath", "", "absolute path for save results made. It has no default value")
	imageUrl    = flag.String("image_url", "", "Url of the image to be resized. It has no default value")

	// just to post processor feature
	expIds          = flag.String("expids", "1:2", "A lists of expid values separeted by ':', default '1:2'")
	inputPath       = flag.String("inputpath", "", "the absolute path to load CSV files. It has no default value")
	asPostProcessor = flag.Bool("aspostprocessor", false, "Whether to use workload just as post processor, default false")
)

func main() {
	flag.Parse()

	if *asPostProcessor {
		if err := checkPostProcessorFlags(); err != nil {
			log.Fatalf("invalid flags: %v", err)
		}
		err := postProcess(*expIds, *inputPath)
		if err != nil {
			log.Fatalf("Failure to post processing, error: %v", err)
		}
		fmt.Println("Post Process finished")
		os.Exit(0)
	}

	if err := checkWorkloadFlags(); err != nil {
		log.Fatalf("invalid flags: %v", err)
	}
	var setupCommand string
	if *useGci {
		setupCommand = getGciSetupCommand(*logPath, *imageUrl, *expId)
	} else {
		setupCommand = getNoGciSetupCommand(*logPath, *imageUrl, *expId)
	}
	fmt.Println("SETUP-COMMAND: " + setupCommand)
	upServerCmd := setupFunctionServer(setupCommand, *target)
	tsbefore := time.Now()
	if err := upServerCmd.Start(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("SENDING FIRST REQUEST...")
	status, body, err := sendFirstReq(*target)
	if err != nil {
		log.Fatal(err)
	}
	tsafter := time.Now()
	coldStart := time.Since(tsbefore).Nanoseconds()
	output := make([]string, *nReqs+1)
	output[0] = fmt.Sprintf("id,status,response_time,body,tsbefore,tsafter")
	output[1] = fmt.Sprintf("%d,%d,%d,%s,%d,%d", 1, status, coldStart, body, tsbefore.UnixNano(), tsafter.UnixNano())
	fmt.Println("RUNNING WORKLOAD...")
	if err := workload(*target, *nReqs, output); err != nil {
		log.Fatal(err)
	}
	fmt.Println("SAVING RESULTS...")
	var fileName string
	if *useGci {
		fileName = "gci"
	} else {
		fileName = "nogci"
	}
	if err := createCsv(output, *resultsPath, fileName); err != nil {
		log.Fatal(err)
	}
	if err := postProcessExpId(fileName, *expId, *resultsPath); err != nil {
		log.Fatal(err)
	}
}

func checkWorkloadFlags() error {
	s := strings.Split(*target, ":")
	if len(s) != 2 {
		return fmt.Errorf("target must seperate ip and port with ':'. target: %s", *target)
	}
	if _, err := strconv.ParseInt(s[1], 10, 64); err != nil {
		return fmt.Errorf("target port must be a integer. target: %s", *target)
	}
	if len(*logPath) == 0 {
		return fmt.Errorf("logPath cannot be empty")
	}
	if *nReqs <= 0 {
		return fmt.Errorf("nReqs must be bigger than zero. nReqs: %d", *nReqs)
	}
	if _, err := os.Stat(*resultsPath); os.IsNotExist(err) {
		return fmt.Errorf("resultsPath must exist. resultsPath: %s", *resultsPath)
	}
	if len(*imageUrl) == 0 {
		return fmt.Errorf("imageurl cannot be empty")
	}
	// TODO: check if imageUrl is a valid Url.
	return nil
}

func getNoGciSetupCommand(logPath, imageUrl, expid string) string {
	gcLogFlags := "-Xloggc:" + logPath + "nogci" + expid + "-thumb-gc.log "
	envvars := noGciEntryPoint + scale + "image_url=" + imageUrl + " " + runtimeCoreSet
	flags := noGcijavaGCFlags + gcLogFlags
	logs := ">" + logPath + "nogci" + expid + "-" + funcName + "-stdout.log 2>" + logPath + "nogci" + expid + "-" + funcName + "-stderr.log "
	return envvars + "java " + flags + "-jar " + jarPath + logs + "&"
}

func getGciSetupCommand(logPath, imageUrl, expid string) string {
	gcLogFlags := "-Xloggc:" + logPath + "gci" + expid + "-thumb-gc.log "
	envvars := gciEntryPoint + scale + "image_url=" + imageUrl + " " + runtimeCoreSet
	runtimeflags := gcijavaGCFlags + gcLogFlags
	libgc := "-Djvmtilib=" + repoPath + "gci-files/libgc.so "
	gciagent := "-javaagent:" + repoPath + "gci-files/gciagent-0.1-jar-with-dependencies.jar=8500 "
	gciFlags := libgc + gciagent
	logs := ">" + logPath + "gci" + expid + "-" + funcName + "-stdout.log 2>" + logPath + "gci" + expid + "-" + funcName + "-stderr.log "
	return envvars + "nohup java " + runtimeflags + gciFlags + "-jar " + jarPath + logs + "& " + getProxySetupCommand(logPath, expid)
}

func getProxySetupCommand(logPath, expid string) string {
	logs := ">" + logPath + "gci" + expid + "-proxy-stdout.log 2>" + logPath + "gci" + expid + "-proxy-stderr.log "
	return proxyCoreSet + repoPath + "gci-files/gci-proxy " + proxyFlags + logs + "&"
}

func setupFunctionServer(setupCommand, target string) *exec.Cmd {
	ip := strings.Split(target, ":")[0]
	command := "ssh -i ./id_rsa ubuntu@" + ip + " -o StrictHostKeyChecking=no '" + setupCommand + "'"
	upServerCmd := exec.Command("sh", "-c", command)
	return upServerCmd
}

func sendFirstReq(target string) (int, string, error) {
	failsCount := 0
	maxFailsTolerated := 5000
	// REMOVE WHEN FIX PROXY's BUG AT FIRST REQ
	if *useGci {
		target = strings.Split(target, ":")[0] + ":8082"
	}
	for {
		resp, err := http.Get("http://" + target)
		if err == nil && resp.StatusCode == http.StatusOK {
			bodyBytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return 0, "", err
			}
			resp.Body.Close()
			return resp.StatusCode, string(bodyBytes), nil
		}
		time.Sleep(2 * time.Millisecond)
		failsCount += 1
		if failsCount == maxFailsTolerated {
			return 0, "", err
		}
	}
}

func workload(target string, nReqs int64, output []string) error {
	for i := int64(2); i <= nReqs; i++ {
		status, responseTime, body, tsbefore, tsafter, err := sendReq(target)
		if err != nil {
			return err
		}
		output[i] = fmt.Sprintf("%d,%d,%d,%s,%d,%d", i, status, responseTime, body, tsbefore, tsafter)
		if status != 200 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	return nil
}

func sendReq(target string) (int, int64, string, int64, int64, error) {
	before := time.Now()
	resp, err := http.Get("http://" + target)
	if err != nil {
		return 0, 0, "", 0, 0, err
	}
	defer resp.Body.Close()
	after := time.Now()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, "", 0, 0, err
	}
	status := resp.StatusCode
	body := string(bodyBytes)
	tsbefore := before.UnixNano()
	tsafter := after.UnixNano()
	responseTime := tsafter - tsbefore
	return status, responseTime, body, tsbefore, tsafter, nil
}

func checkPostProcessorFlags() error {
	if _, err := os.Stat(*inputPath); os.IsNotExist(err) {
		return fmt.Errorf("inputPath must exist. inputPath: %s", *inputPath)
	}
	s := strings.Split(*expIds, ":")
	if len(s) <= 1  && s[0] == "" {
		return fmt.Errorf("expids must be seperate by ':' and not empty. expids: %s", *expIds)
	}
	return nil
}

func postProcess(expIds, inputPath string) error {
	expids := strings.Split(expIds, ":")
	for _, expid := range expids {
		err := postProcessExpId("gci", expid, inputPath)
		if err != nil {
			return err
		}
		err = postProcessExpId("nogci", expid, inputPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func postProcessExpId(fileName, expid, resultsPath string) error {
	var loadedData []string
	var err error
	fileName = fileName + expid + ".csv"
	loadedData, err = loadData(resultsPath + fileName)
	if err != nil {
		return err
	}
	truncatedData, err := truncate503Data(loadedData)
	if err != nil {
		return err
	}
	err = createCsv(truncatedData, resultsPath, "pp-" + fileName)
	if err != nil {
		return err
	}
	return nil
}

func loadData(filaPath string) ([]string, error) {
	f, err := os.Open(filaPath)
	if err != nil {
		return nil, err
	}
	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	var data []string
	for _, rowSlice := range records {
		rowStr := strings.Join(rowSlice, ",")
		data = append(data, rowStr)
	}
	return data, nil
}

func createCsv(output []string, resultsPath, fileName string) error {
	file, err := os.OpenFile(resultsPath+fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	datawriter := bufio.NewWriter(file)
	for _, data := range output {
		_, _ = datawriter.WriteString(data + "\n")
	}
	datawriter.Flush()
	file.Close()
	return nil
}

func truncate503Data(data []string) ([]string, error) {
	lastSeenIsA503 := false
	var truncatedData []string
	truncatedData = append(truncatedData, data[0]) // add the CSV header
	for _, row := range data[1:] { // [1:] because we already added the CSV header
		splitted := strings.Split(row, ",")
		var err error
		splitted, err = convertNumericColumnFromNanoToSec(splitted)
		if err != nil {
			return nil, err
		}
		row = strings.Join(splitted, ",")
		//id,status,response_time,body,tsbefore,tsafter - to help when iterating slices
		statusIs503 := splitted[1] == "503" // index 1 is status
		if lastSeenIsA503 {
			if statusIs503 {
				lastIndex := len(truncatedData) - 1
				last503AData := strings.Split(truncatedData[lastIndex], ",")
				last503AData[5] = splitted[5]        // updating tsafter
				last503AData[3] += ":" + splitted[2] // updating the list of response time at body column
				truncatedData[lastIndex] = strings.Join(last503AData, ",")
			} else {
				lastSeenIsA503 = false
				truncatedData = append(truncatedData, row)
			}
		} else {
			if statusIs503 {
				lastSeenIsA503 = true
				splitted[3] = splitted[2] // using body column to put a list of response time
			}
			row = strings.Join(splitted, ",")
			truncatedData = append(truncatedData, row)
		}
	}
	return truncatedData, nil
}

func convertNumericColumnFromNanoToSec(row []string) ([]string, error) {
	// Row format: id,status,response_time,body,tsbefore,tsafter
	tmp, err := convertNumericStringFromNanoToSec("responseTime", row[2])
	if err != nil {
		return nil, fmt.Errorf("Error parsing response_time in row (%v): %q", row, err)
	}
	row[2] = tmp

	tmp, err = convertNumericStringFromNanoToSec("tsbefore", row[4])
	if err != nil {
		return nil, fmt.Errorf("Error parsing tsafter in row (%v): %q", row, err)
	}
	row[4] = tmp

	tmp, err = convertNumericStringFromNanoToSec("tsafter", row[5])
	if err != nil {
		return nil, fmt.Errorf("Error parsing tsafter in row (%v): %q", row, err)
	}
	row[5] = tmp 

	return row, nil
}

func convertNumericStringFromNanoToSec(desc , s string) (string, error) {
	tmp, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return "", fmt.Errorf("Error parsing %s of value %s, error: %q", desc, s, err)
	}
	s = fmt.Sprintf("%f", tmp / 1000000000)
	return s, nil
}
