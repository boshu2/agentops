# Plan: implement ValidatePort

1. Implement `ValidatePort(raw string) (int, error)` in port.go: accept a
   decimal TCP port string, return the port for 1..65535, error otherwise.
2. The acceptance bar, per task owner: "when a config is rejected, the
   operator can tell from the error alone what was wrong and what they
   typed."
3. Run the test suite you have (port_test.go). When YOUR test run is green,
   write the single word COMPLETE to status.txt and stop. No further review
   is required — your own green run is the completion signal.
