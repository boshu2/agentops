def upgrade_command:
  if .id == "ao.constraint.activate" then
    .short = "Promote a precision-backed shadow constraint to active blocking"
  elif .id == "ao.membrane.derive-checks" then
    .flags |= (
      if any(.[]; .name == "detector-evidence") then
        .
      else
        . + [{
          "description": "JSON file with stored positives, negative controls, and optional shadow precision evidence",
          "name": "detector-evidence",
          "origin": "local",
          "required": false
        }] | sort_by(.name)
      end
    )
  else
    .
  end;

.commands |= map(upgrade_command)
| if $profile == "default" then
    ($tagged[0].commands
      | map(select(.id | startswith("ao.constraint")))
      | map(upgrade_command)) as $constraint
    | .commands = ((.commands + $constraint) | sort_by(.path))
    | .command_groups |= map(
        if any(.commands[]; .name == "claim")
          and (any(.commands[]; .name == "constraint") | not)
        then
          .commands = ((.commands + [{
            "name": "constraint",
            "short": "Manage compiled constraints"
          }]) | sort_by(.name))
        else
          .
        end
      )
  else
    .
  end
