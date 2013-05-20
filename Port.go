package main
 
import (
    "fmt"
    "net"
    "time"
    "strings"
    "strconv"
    "io"
    "os"
    "bufio"
)

type ipf struct {
    ip string
    port uint16
    flag bool
}

type Exception struct {
    message string
    code int
}
func (e Exception) Error() string {
    return e.message
}

func IpSformat(ip int) (string, error) {
    return fmt.Sprintf("%d.%d.%d.%d", uint8(ip >> 24), uint8(ip >> 16), uint8(ip >> 8), uint8(ip)) , nil
}

func IpIformat(ip string) (int, error) {
    ipSplit := strings.Split(ip, ".")
    if len(ipSplit) != 4 {
        return 0, Exception{"IP Parse Error", -10}
    }
    p1, err := strconv.Atoi(ipSplit[0])
    if err != nil {
        return 0, Exception{"IP Parse Error", -10}
    }
    p2, err := strconv.Atoi(ipSplit[1])
    if err != nil {
        return 0, Exception{"IP Parse Error", -10}
    }
    p3, err := strconv.Atoi(ipSplit[2])
    if err != nil {
        return 0, Exception{"IP Parse Error", -10}
    }
    p4, err := strconv.Atoi(ipSplit[3])
    if err != nil {
        return 0, Exception{"IP Parse Error", -10}
    }
    return p1 << 24 + p2 << 16 + p3 << 8 + p4, nil
}

type Config struct{
    records ipTables
    running int
    timeOut int
}

type ipTables map[string]bool

func (c *Config) Parser(fileName string) {
    c.records = make(ipTables)
    c.timeOut = 2
    // open file
    file, err := os.Open(fileName)
    if err != nil {
        panic("File not found")
    }
    reader := bufio.NewReader(file)
    var b byte

    token, key, value, lLeftRange, lRightRange, rLeftRange, rRightRange := "", "", "", "", "", "", ""
    defaultPort, lineNum, status := 80, 1, 0

    for {
        eof := false
        b, err = reader.ReadByte()
        if err == io.EOF && status == 0 {
            break
        }else if err == io.EOF && !eof {
            eof = true
            b = '\r'
        }else if err != nil {
            panic(fmt.Sprintf("Line %d: ", lineNum))
        }
        if b == ' ' || b == '\t' || b == '\n' {
            continue
        }
        switch status {
        case -1: // comment
            if b == '\r' {
                key = "#"
                status = 100
                reader.UnreadByte()
            }
        case 0: // start
            token, key, value, lLeftRange, lRightRange, rLeftRange, rRightRange = "", "", "", "", "", "", ""
            if b == '#' {
                status = -1
            }else if b == ';' || b == '\r' {
                key = "#"
                status = 100
                reader.UnreadByte()
            }else if b < '0' || b > '9' {
                status = 5
                token += string(b)
            }else{
                status = 1
                token += string(b)
            }
        case 1: // lLeftRange
            if b == '\r' || b == ';' || b == '#' {
                status = 100
                lLeftRange = token
                reader.UnreadByte()
            }else if b == ':' {
                status = 2
                lLeftRange = token
                token = ""
            }else if b == '-' {
                status = 3
                lLeftRange = token
                token = ""
            }else if (b < '0' || b > '9') && b != '.' {
                panic(fmt.Sprintf("Line %d: Invalid IP", lineNum))
            }else{
                token += string(b)
            }
        case 2: // rLeftRange
            if b == '\r' || b == ';' || b == '#' {
                status = 100
                rLeftRange =  token
                reader.UnreadByte()
            }else if b == '-' {
                status = 4
                rLeftRange = token
                token = ""
            }else if b < '0' || b > '9' {
                panic(fmt.Sprintf("Line %d: Invalid Port", lineNum))
            }else{
                token += string(b)
            }
        case 3: // lRightRange
            if b == '\r' || b == ';' || b == '#' {
                status = 100
                lRightRange =  token
                reader.UnreadByte()
            }else if b == ':' {
                status = 2
                lRightRange = token
                token = ""
            }else if (b < '0' || b > '9') && b != '.' {
                panic(fmt.Sprintf("Line %d: Invalid IP", lineNum))
            }else{
                token += string(b)
            }
        case 4: // rRightRange
            if b == '\r' || b == ';' || b == '#' {
                status = 100
                rRightRange =  token
                reader.UnreadByte()
            }else if b < '0' || b > '9' {
                panic(fmt.Sprintf("Line %d: Invalid Port", lineNum))
            }else{
                token += string(b)
            }
        case 5: // config-key
            if b == ':' {
                status = 6
                key = token
                token = ""
            }else{
                token += string(b)
            }
        case 6: // config-value
            if token != "" && ( b == '\r' || b == ';' || b == '#') {
                status = 100
                value = token
                reader.UnreadByte()
            }else if token == "" && ( b == '\r' || b == ';') {
                panic(fmt.Sprintf("Line %d: Empty Value", lineNum))
            }else{
                token += string(b)
            }
        case 100: // expression end
            if key != "" { // config
                switch strings.ToLower(key) {
                case "#":
                case "timeout":
                    c.timeOut, err = strconv.Atoi(value)
                    if err != nil || c.timeOut < 1 {
                        panic(fmt.Sprintf("Line %d: Invalid Timeout", lineNum))
                    }
                case "port":
                    defaultPort, err = strconv.Atoi(value)
                    if err != nil || defaultPort < 0 || defaultPort > 65535 {
                        panic(fmt.Sprintf("Line %d: Invalid Port", lineNum))
                    }
                default:
                    // panic(fmt.Sprintf("Line %d: Invalid Config", lineNum))
                }
            }else{
                startIp, endIp, startPort, endPort := 0, 0, 0, 0
                // map ip
                if lRightRange != "" { // range ip
                    startIp, err = IpIformat(lLeftRange)
                    if err != nil {
                        panic(fmt.Sprintf("Line %d: Invalid IP", lineNum))
                    }
                    endIp, err = IpIformat(lRightRange)
                    if err != nil {
                        panic(fmt.Sprintf("Line %d: Invalid IP", lineNum))
                    }
                    if startIp > endIp {
                        panic(fmt.Sprintf("Line %d: Invalid IP Range", lineNum))
                    }
                }else{ // single ip
                    startIp, err = IpIformat(lLeftRange)
                    if err != nil {
                        panic(fmt.Sprintf("Line %d: Invalid IP", lineNum))
                    }
                    endIp = startIp
                }

                // map port
                if rLeftRange == "" { // default port
                    startPort = defaultPort
                    endPort = startPort
                }else if rRightRange == "" { // single port
                    startPort, err = strconv.Atoi(rLeftRange)
                    if err != nil {
                        panic(fmt.Sprintf("Line %d: Invalid Port", lineNum))
                    }
                    endPort = startPort
                }else{ // range port
                    startPort, err = strconv.Atoi(rLeftRange)
                    if err != nil {
                        panic(fmt.Sprintf("Line %d: Invalid Port", lineNum))
                    }
                    endPort, err = strconv.Atoi(rRightRange)
                    if err != nil {
                        panic(fmt.Sprintf("Line %d: Invalid Port", lineNum))
                    }
                }
                if startPort < 0 || startPort > 65535 || endPort < 0 || endPort > 65535 || startPort > endPort {
                    panic(fmt.Sprintf("Line %d: Invalid Port Range", lineNum))
                }

                // create map
                for ip := startIp; ip <= endIp; ip++ {
                    ipStr, err := IpSformat(ip)
                    if err != nil {
                        panic(fmt.Sprintf("Line %d: Invalid Ip", lineNum))
                    }
                    for port := startPort; port <= endPort; port++ {
                        c.records[ipStr + fmt.Sprintf(":%d", port)] = false
                    }
                }
            }
            if b == '\r' {
                lineNum++
            }
            if status == 100 {
                status = 0
            }
        default:
            panic(fmt.Sprintf("Line %d: Unknown Config Status", lineNum))
        }
    }
}

var config Config

func main() {
    defer func() {
        if e := recover(); e != nil {
            fmt.Println(e)
        }
    }()    
    config.Parser("config")
    CheckPort(&config)
    fmt.Println(config.records)
}

func CheckPort(c *Config) {
    for record := range c.records {
        for c.running >= 5 {
            time.Sleep(1 * time.Second)
        }
        r := strings.Split(record, ":")
        port,_ := strconv.Atoi(r[1])
        c.running++
        go check(c, r[0], uint16(port))
    }

    for c.running != 0 {
        time.Sleep(1 * time.Second)
    }
}

func check(c *Config, ip string, port uint16) {
    connection, err := net.DialTimeout("tcp", ip + ":" + fmt.Sprintf("%d", port), time.Duration(c.timeOut) * time.Second)
    if err == nil {
        c.records[fmt.Sprintf("%s:%d", ip, port)] = true
        fmt.Println(fmt.Sprintf("%s:%d - true", ip, port))
        connection.Close()
    }else{
        c.records[fmt.Sprintf("%s:%d", ip, port)] = false
        fmt.Println(fmt.Sprintf("%s:%d - %s", ip, port, err))
    }
    c.running--
}
