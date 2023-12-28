package main

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
    "flag"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	probing "github.com/prometheus-community/pro-bing"
	"golang.org/x/term"
)

type pingStats struct {
	Name string
	Addr string

	pktsSent  int
	pktsRecv  int
	rttAvg    time.Duration
	rttVar    float64
	rttStdDev time.Duration

	mtx sync.Mutex
}

func newPingStats(name, addr string) *pingStats {
	return &pingStats{
		Name: name,
		Addr: addr,
	}
}

func (s *pingStats) PktLoss() float64 {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return float64(s.pktsSent-s.pktsRecv) / float64(s.pktsSent) * 100
}

func (s *pingStats) OnPktSend(pkt *probing.Packet) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.pktsSent++
}

func (s *pingStats) OnPktRecv(pkt *probing.Packet) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.pktsRecv++
	// Welford's online algorithm for std dev
	// https://en.wikipedia.org/wiki/Algorithms_for_calculating_variance#Welford's_online_algorithm
	if s.pktsRecv == 1 {
		s.rttAvg = pkt.Rtt
		s.rttVar = 0
	} else {
		delta := pkt.Rtt - s.rttAvg
		s.rttAvg += delta / time.Duration(s.pktsRecv)
		s.rttVar += float64(delta) * float64(pkt.Rtt-s.rttAvg)
		s.rttStdDev = time.Duration(math.Sqrt(s.rttVar / float64(s.pktsRecv-1)))
	}
}

func (s *pingStats) RttAvg() time.Duration {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.rttAvg
}

func (s *pingStats) RttStdDev() time.Duration {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	return s.rttStdDev
}

func pingsToRows(pings []*pingStats) []table.Row {
	rows := make([]table.Row, len(pings))
	for i, s := range pings {
		rows[i] = table.Row{
            s.Name,
            s.Addr,
            fmt.Sprintf("%.1f%%", s.PktLoss()), // Packet Loss
            s.RttAvg().Round(100 * time.Microsecond).String(), // RTT Avg
            s.RttStdDev().Round(100 * time.Microsecond).String(), // RTT Std Dev
        }
	}
	return rows
}

func runPing(s *pingStats) error {
	pinger, err := probing.NewPinger(s.Addr)
	if err != nil {
		return err
	}
	pinger.OnSend = s.OnPktSend
	pinger.OnRecv = s.OnPktRecv
	return pinger.Run()
}

func readPingTargets(path string) []*pingStats {
	stats := make([]*pingStats, 0)

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("Error reading config file:", err)
		os.Exit(1)
	}
	scanner := bufio.NewScanner(bytes.NewReader(fileBytes))
	for scanner.Scan() {
		line := scanner.Text()
		name, addr, _ := strings.Cut(line, "|")
		stats = append(stats, newPingStats(name, addr))
	}

	return stats
}

type model struct {
	table table.Model
	pings []*pingStats
}

type updateRowsMsg struct{}

func updateRowsCmd() tea.Msg {
	time.Sleep(1 * time.Second)
	return updateRowsMsg{}
}

func (m model) Init() tea.Cmd { return updateRowsCmd }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case updateRowsMsg:
		sort.Slice(m.pings, func(i, j int) bool {
			return m.pings[i].RttAvg() < m.pings[j].RttAvg()
		})
		m.table.SetRows(pingsToRows(m.pings))
		return m, updateRowsCmd
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

func (m model) View() string {
	return baseStyle.Render(m.table.View()) + "\n"
}

func newTable(cols []table.Column, rows []table.Row) table.Model {
	_, termHeight, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		termHeight = 0
	}
	tableHeight := max(termHeight-8, 1)

	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	styles.Selected = styles.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	return table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(tableHeight),
		table.WithStyles(styles),
	)
}

func main() {
    flag.Parse()

	pings := readPingTargets(flag.Arg(0))
	for _, s := range pings {
		go runPing(s)
	}

	cols := []table.Column{
		{Title: "Name", Width: 30},
		{Title: "Address", Width: 30},
		{Title: "Packet Loss", Width: 15},
		{Title: "RTT Avg", Width: 15},
		{Title: "RTT Std Dev", Width: 15},
	}
	rows := pingsToRows(pings)
	m := model{newTable(cols, rows), pings}

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
