#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 1 ]] || { echo 'scope output: usage: check-output.sh OUTPUT.yaml' >&2; exit 2; }
output=$1
[[ -f "$output" && ! -L "$output" ]] || { echo 'scope output: input must be a regular non-symlink file' >&2; exit 2; }
bytes=$(wc -c <"$output" | tr -d ' ')
(( bytes > 0 && bytes <= 262144 )) || { echo 'scope output: input must be 1-262144 bytes' >&2; exit 2; }

ruby - "$output" <<'RUBY'
require "yaml"

def stop(message)
  warn "scope output: #{message}"
  exit 2
end

path = ARGV.fetch(0)
begin
  doc = YAML.safe_load(File.read(path), permitted_classes: [], permitted_symbols: [], aliases: false)
rescue StandardError => e
  stop("invalid safe YAML: #{e.message}")
end
stop("document must be a mapping") unless doc.is_a?(Hash)
required = %w[decision write_scope generated_companions axioms gaps ambiguities checked not_checked]
stop("keys must be exactly #{required.join(', ')}") unless doc.keys.sort == required.sort
stop("decision must be COMPLETE or NEEDS_INPUT") unless %w[COMPLETE NEEDS_INPUT].include?(doc["decision"])
scope = doc["write_scope"]
stop("write_scope must contain exactly include and exclude") unless scope.is_a?(Hash) && scope.keys.sort == %w[exclude include]

lists = {
  "include" => scope["include"],
  "exclude" => scope["exclude"],
  "generated" => doc["generated_companions"],
}
lists.each do |kind, values|
  stop("#{kind} must be an array of unique strings") unless values.is_a?(Array) && values.all? { |v| v.is_a?(String) } && values.uniq.length == values.length
end
stop("include must contain at least one bounded pattern") if lists["include"].empty?

all_refs = []
lists.each do |kind, values|
  values.each do |pattern|
    stop("#{kind} pattern is empty or too long") if pattern.empty? || pattern.bytesize > 256
    stop("#{kind} pattern must be repository-relative: #{pattern}") if pattern.start_with?("/", "~", "-", "*")
    stop("#{kind} pattern contains a non-normal path form: #{pattern}") if pattern.include?("\\") || pattern.include?("//") || pattern.end_with?("/")
    parts = pattern.split("/")
    stop("#{kind} pattern traverses or aliases a directory: #{pattern}") if parts.any? { |part| part.empty? || part == "." || part == ".." }
    stop("#{kind} pattern contains control characters: #{pattern}") if pattern.match?(/[[:cntrl:]]/)
    all_refs << "#{kind}:#{pattern}"
  end
end
stop("the same exact pattern appears in more than one scope class") unless all_refs.uniq.length == all_refs.length

axioms = doc["axioms"]
stop("axioms must be a nonempty array") unless axioms.is_a?(Array) && !axioms.empty?
mapped = []
axioms.each_with_index do |axiom, index|
  stop("axiom #{index + 1} must contain exactly fact and patterns") unless axiom.is_a?(Hash) && axiom.keys.sort == %w[fact patterns]
  stop("axiom #{index + 1} needs a factual sentence") unless axiom["fact"].is_a?(String) && !axiom["fact"].strip.empty?
  refs = axiom["patterns"]
  stop("axiom #{index + 1} needs one or more pattern references") unless refs.is_a?(Array) && !refs.empty? && refs.all? { |v| v.is_a?(String) }
  mapped.concat(refs)
end
unknown = mapped - all_refs
missing = all_refs - mapped
duplicate = mapped.group_by(&:itself).select { |_key, values| values.length != 1 }.keys
stop("axioms reference unknown patterns: #{unknown.join(', ')}") unless unknown.empty?
stop("patterns lack an axiom: #{missing.join(', ')}") unless missing.empty?
stop("patterns map to more than one axiom: #{duplicate.join(', ')}") unless duplicate.empty?

%w[gaps ambiguities checked not_checked].each do |field|
  value = doc[field]
  stop("#{field} must be an array of unique nonempty strings") unless value.is_a?(Array) && value.all? { |v| v.is_a?(String) && !v.strip.empty? } && value.uniq.length == value.length
end
stop("checked must name at least one inspected acceptance/source fact") if doc["checked"].empty?
if doc["decision"] == "COMPLETE"
  stop("COMPLETE requires empty gaps and ambiguities") unless doc["gaps"].empty? && doc["ambiguities"].empty?
else
  stop("NEEDS_INPUT must name a gap or ambiguity") if doc["gaps"].empty? && doc["ambiguities"].empty?
end

puts "scope output: PASS (#{doc['decision']}, #{all_refs.length} mapped patterns)"
RUBY
