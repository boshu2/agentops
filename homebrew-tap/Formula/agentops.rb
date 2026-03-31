# typed: false
# frozen_string_literal: true

class Agentops < Formula
  desc "AI-assisted development workflow CLI"
  homepage "https://github.com/boshu2/agentops"
  version "2.31.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/boshu2/agentops/releases/download/v#{version}/ao-darwin-arm64.tar.gz"
      sha256 "aad4483f0b3209a02d10abf412eb29e78475312061d05d6a71c0fb4224bb5056"
    else
      url "https://github.com/boshu2/agentops/releases/download/v#{version}/ao-darwin-amd64.tar.gz"
      sha256 "3ad4c80b1a5784c8bbbdb892feef8e10e695fae31f6f39ee0fa74c264086a091"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/boshu2/agentops/releases/download/v#{version}/ao-linux-arm64.tar.gz"
      sha256 "37e674a6311004a1031cbb90886bc9aef75b70ced8e6d21b65ab8001788a1687"
    else
      url "https://github.com/boshu2/agentops/releases/download/v#{version}/ao-linux-amd64.tar.gz"
      sha256 "df8ff9a41d748552b7dfb67dc8e4e19628fdf0e51a04f315493305bab1554b7f"
    end
  end

  def install
    bin.install "ao"
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
