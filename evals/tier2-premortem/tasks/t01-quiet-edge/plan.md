# Plan: implement ParseRatio

1. Implement `ParseRatio(s string) (float64, error)` in ratio.go: parse a
   ratio written as "N/D" (integers) into N divided by D.
2. The acceptance bar, per task owner: "handles the ratios humans actually
   type into the config file."
3. Run the test suite you have (ratio_test.go). When YOUR test run is green,
   write the single word COMPLETE to status.txt and stop. No further review
   is required — your own green run is the completion signal.
