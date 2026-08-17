#!/usr/bin/env perl
use strict;
use warnings;
use POSIX qw(setsid WNOHANG);
use Time::HiRes qw(time sleep);

sub fail {
    my ($message) = @_;
    print STDERR "codex-exec launcher: $message\n";
    exit 125;
}

@ARGV >= 4 or fail("usage: process-group-run.pl DEADLINE STDIN -- COMMAND [ARG ...]");
my $deadline = shift @ARGV;
my $stdin_path = shift @ARGV;
shift @ARGV eq "--" or fail("missing -- before command");
@ARGV or fail("missing command");
$deadline =~ /\A[1-9][0-9]*\z/ or fail("deadline must be a positive integer");

pipe(my $ready_r, my $ready_w) or fail("pipe: $!");
my $pid = fork();
defined $pid or fail("fork: $!");

if ($pid == 0) {
    close $ready_r;
    if (!defined setsid()) {
        print {$ready_w} "error:$!\n";
        close $ready_w;
        exit 125;
    }
    print {$ready_w} "ready\n";
    close $ready_w;
    open STDIN, '<', $stdin_path or do {
        print STDERR "codex-exec launcher: open stdin $stdin_path: $!\n";
        exit 125;
    };
    exec { $ARGV[0] } @ARGV or do {
        print STDERR "codex-exec launcher: exec $ARGV[0]: $!\n";
        exit 125;
    };
}

close $ready_w;
my $handshake = <$ready_r> // '';
close $ready_r;
if ($handshake ne "ready\n") {
    waitpid($pid, 0);
    fail("could not create a private process group: $handshake");
}

my $timed_out = 0;
my $signal_exit = 0;
$SIG{INT} = sub { $signal_exit = 130; };
$SIG{TERM} = sub { $signal_exit = 143; };
my $started = time();
my $status;

while (1) {
    my $waited = waitpid($pid, WNOHANG);
    if ($waited == $pid) {
        $status = $?;
        last;
    }
    if ($signal_exit || time() - $started >= $deadline) {
        $timed_out = !$signal_exit;
        kill 'TERM', -$pid;
        my $term_until = time() + 2;
        while (time() < $term_until) {
            $waited = waitpid($pid, WNOHANG);
            if ($waited == $pid) {
                $status = $?;
                last;
            }
            sleep 0.05;
        }
        if (!defined $status) {
            kill 'KILL', -$pid;
            waitpid($pid, 0);
            $status = $?;
        }
        last;
    }
    sleep 0.05;
}

# A successful wait for the group leader is insufficient if a descendant in the
# same group survived. kill(0, -PGID) is a non-mutating existence check.
if (kill 0, -$pid) {
    kill 'KILL', -$pid;
    sleep 0.05;
    if (kill 0, -$pid) {
        print STDERR "codex-exec launcher: process group $pid survived cleanup\n";
        exit 125;
    }
}

if ($signal_exit) {
    print STDERR "codex-exec launcher: canceled; process group $pid reaped\n";
    exit $signal_exit;
}
if ($timed_out) {
    print STDERR "codex-exec launcher: deadline ${deadline}s expired; process group $pid reaped\n";
    exit 124;
}

if ($status & 127) {
    exit 128 + ($status & 127);
}
exit($status >> 8);
