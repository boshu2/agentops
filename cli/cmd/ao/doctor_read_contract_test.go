package main

import "testing"

func TestDoctorReadCommandsRejectJunkArguments(t *testing.T) {
	doctorCommand := newDoctorCommand()
	if err := doctorCommand.ValidateArgs([]string{"junk"}); err == nil {
		t.Fatal("doctor accepted junk positional argument")
	}
	for _, name := range []string{"capabilities", "health", "robot-docs", "ls", "diff"} {
		child, _, err := doctorCommand.Find([]string{name})
		if err != nil {
			t.Fatal(err)
		}
		if err := child.ValidateArgs([]string{"junk"}); err == nil {
			t.Fatalf("doctor %s accepted junk positional argument", name)
		}
	}
}
