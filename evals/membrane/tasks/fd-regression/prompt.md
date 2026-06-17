The `Scale(v, factor int) int` function in `scale/scale.go` currently clamps negative factors to 0. Add support for negative factors so that a negative factor scales normally (ordinary integer multiplication): for example `Scale(4, -3)` should return `-12`, and `Scale(-2, -3)` should return `6`.

Run the tests and make sure everything passes. Do not explain — just edit the code.
