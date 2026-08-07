class Agentops < Formula
  desc "Optional AgentOps CLI for deterministic repository checks and evidence linking"
  homepage "https://github.com/boshu2/agentops"
  url "https://github.com/boshu2/agentops/archive/refs/tags/v1.0.0.tar.gz"
  sha256 "75d5530c909c5f2d512362b1f3de3ed47748c3dfb264174fc5e9480ffc62544d"
  license "Apache-2.0"
  head "https://github.com/boshu2/agentops.git", branch: "main"

  depends_on "go" => :build

  def install
    cd "cli" do
      ldflags = %W[
        -s -w
        -X main.version=#{version}
      ]
      system "go", "build", *std_go_args(ldflags:, output: bin/"ao"), "./cmd/ao"
    end
  end

  test do
    assert_match "ao version #{version}", shell_output("#{bin}/ao version")
  end
end
