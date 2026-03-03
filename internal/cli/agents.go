package cli

import "fmt"

func runAgents(args []string) {
	project, _ := parseProject(args)

	d := openDB()
	defer d.Close()

	agents, err := d.ListAgents(project)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	if len(agents) == 0 {
		fmt.Println("no agents registered")
		return
	}

	rows := [][]string{{"", "NAME", "ROLE", "LAST SEEN"}}
	for _, a := range agents {
		rows = append(rows, []string{
			statusIndicator(a.LastSeen),
			a.Name,
			truncate(a.Role, 30),
			timeAgo(a.LastSeen),
		})
	}
	printTable(rows)
}
