# typed: false
# frozen_string_literal: true

class Agentops < Formula
  desc "AI-assisted development workflow CLI"
  homepage "https://github.com/boshu2/agentops"
  version "1.0.0"
  license "MIT"

  # TODO: Update URL when ao CLI is released
  # url "https://github.com/boshu2/agentops/releases/download/v#{version}/ao-#{version}-darwin-amd64.tar.gz"
  # sha256 "PLACEHOLDER"

  # For development, build from source
  head "https://github.com/boshu2/agentops.git", branch: "main"

  depends_on "go" => :build

  def install
    # Build ao CLI from cli/ directory
    cd "cli" do
      system "go", "build", *std_go_args(ldflags: "-s -w -X main.version=#{version}"), "./cmd/ao"
    end
  end

  def caveats
    <<~EOS
      AgentOps ao CLI installed!

      Commands:
        ao forge transcript <path>  # Extract from JSONL transcripts
        ao forge markdown <path>   # Extract from markdown files
        ao ratchet record <type>   # Record progress
        ao ratchet verify <epic>   # Verify completion

      For the Claude Code plugin, run:
        claude /plugin add boshu2/agentops
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/ao --version")
  end
end
