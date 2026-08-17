#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 1 ]] || { echo 'standards findings: usage: check-findings.sh FINDINGS.json' >&2; exit 2; }
input=$1
[[ -f "$input" && ! -L "$input" ]] || { echo 'standards findings: input must be a regular non-symlink file' >&2; exit 2; }
bytes=$(wc -c <"$input" | tr -d ' ')
(( bytes > 0 && bytes <= 262144 )) || { echo 'standards findings: input must be 1-262144 bytes' >&2; exit 2; }
skill_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)

ruby -rjson - "$input" "$skill_dir" <<'RUBY'
def stop(message)
  warn "standards findings: #{message}"
  exit 2
end

input, skill_dir = ARGV
begin
  doc = JSON.parse(File.read(input))
rescue StandardError => e
  stop("invalid JSON: #{e.message}")
end
stop("document must be an object") unless doc.is_a?(Hash)
required = %w[decision change selected_references reference_reasons checked not_checked findings]
stop("keys must be exactly #{required.join(', ')}") unless doc.keys.sort == required.sort
stop("decision must be COMPLETE or INCOMPLETE") unless %w[COMPLETE INCOMPLETE].include?(doc["decision"])

change = doc["change"]
stop("change must contain exactly paths, language, change_type, and risk_cues") unless change.is_a?(Hash) && change.keys.sort == %w[change_type language paths risk_cues]
paths = change["paths"]
stop("change.paths must be a nonempty unique string array") unless paths.is_a?(Array) && !paths.empty? && paths.all? { |v| v.is_a?(String) } && paths.uniq.length == paths.length
paths.each do |path|
  stop("unsafe or empty change path: #{path.inspect}") if path.empty? || path.bytesize > 512 || path.start_with?("/", "~", "-") || path.end_with?("/") || path.include?("\\") || path.include?("//") || path.split("/").any? { |part| part == "." || part == ".." } || path.match?(/[[:cntrl:]]/)
end

language_refs = {
  "go" => "references/go.md", "python" => "references/python.md", "rust" => "references/rust.md",
  "typescript" => "references/typescript.md", "javascript" => "references/javascript.md",
  "shell" => "references/shell.md", "json" => "references/json.md", "yaml" => "references/yaml.md",
  "markdown" => "references/markdown.md", "none" => nil,
}
language = change["language"]
stop("unsupported language: #{language.inspect}") unless language_refs.key?(language)
stop("change_type must be a nonempty bounded string") unless change["change_type"].is_a?(String) && change["change_type"].match?(/\A[a-z][a-z0-9_-]{0,63}\z/)

risks = change["risk_cues"]
allowed_risks = %w[none bulk_rewrite sql concurrency untrusted_llm_output test_strategy skill_package]
stop("risk_cues must be a unique array from the declared vocabulary") unless risks.is_a?(Array) && !risks.empty? && risks.all? { |v| allowed_risks.include?(v) } && risks.uniq.length == risks.length
stop("none cannot be combined with another risk cue") if risks.include?("none") && risks.length != 1

refs = doc["selected_references"]
stop("selected_references must be a unique array beginning with common standards") unless refs.is_a?(Array) && refs.uniq.length == refs.length && refs.first == "references/common-standards.md"
ref_root = File.realpath(File.join(skill_dir, "references"))
refs.each do |ref|
  stop("unsafe reference path: #{ref.inspect}") unless ref.is_a?(String) && ref.match?(/\Areferences\/[A-Za-z0-9._-]+\.md\z/)
  full = File.join(skill_dir, ref)
  stop("selected reference does not exist: #{ref}") unless File.file?(full) && !File.symlink?(full)
  stop("selected reference escapes package: #{ref}") unless File.realpath(full).start_with?(ref_root + File::SEPARATOR)
end
stop("selected references omit language owner #{language_refs[language]}") if language_refs[language] && !refs.include?(language_refs[language])

risk_refs = {
  "sql" => "references/sql-safety-checklist.md",
  "concurrency" => "references/race-condition-checklist.md",
  "untrusted_llm_output" => "references/llm-trust-boundary-checklist.md",
  "test_strategy" => "references/test-pyramid.md",
  "skill_package" => "references/skill-structure.md",
}
risk_refs.each do |risk, ref|
  stop("risk cue #{risk} requires #{ref}") if risks.include?(risk) && !refs.include?(ref)
end

reasons = doc["reference_reasons"]
stop("reference_reasons must explain exactly the selected references") unless reasons.is_a?(Hash) && reasons.keys.sort == refs.sort && reasons.values.all? { |v| v.is_a?(String) && !v.strip.empty? }
%w[checked not_checked].each do |field|
  values = doc[field]
  stop("#{field} must be an array of unique nonempty strings") unless values.is_a?(Array) && values.all? { |v| v.is_a?(String) && !v.strip.empty? } && values.uniq.length == values.length
end
stop("checked must name at least one inspected surface") if doc["checked"].empty?
if doc["decision"] == "COMPLETE"
  stop("COMPLETE requires empty not_checked") unless doc["not_checked"].empty?
else
  stop("INCOMPLETE requires at least one not_checked surface") if doc["not_checked"].empty?
end

if risks.include?("bulk_rewrite") || change["change_type"] == "bulk_rewrite"
  required_checks = %w[mutation-chokepoint hash-witnessed-backup ambition-gate]
  missing = required_checks - doc["checked"]
  stop("bulk rewrite omitted mandatory checks: #{missing.join(', ')}") unless missing.empty?
end

findings = doc["findings"]
stop("findings must be an array") unless findings.is_a?(Array)
findings.each_with_index do |finding, index|
  stop("finding #{index + 1} has the wrong keys") unless finding.is_a?(Hash) && finding.keys.sort == %w[line message path reference section severity]
  stop("finding #{index + 1} has invalid severity") unless %w[error warning info].include?(finding["severity"])
  stop("finding #{index + 1} path was not supplied") unless paths.include?(finding["path"])
  stop("finding #{index + 1} needs a positive line") unless finding["line"].is_a?(Integer) && finding["line"] >= 1
  stop("finding #{index + 1} cites an unselected reference") unless refs.include?(finding["reference"])
  stop("finding #{index + 1} needs a section and concrete message") unless finding["section"].is_a?(String) && !finding["section"].strip.empty? && finding["message"].is_a?(String) && finding["message"].strip.length >= 8
end

puts "standards findings: PASS (#{doc['decision']}, #{refs.length} references, #{findings.length} findings)"
RUBY
