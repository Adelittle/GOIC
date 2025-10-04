<div align="center">
<h1>GOIC - Go Orbit Ion Cannon</h1>
<p><strong>Golang Web-Based HTTP stress testing / DDOS tool inspired by HOIC.</strong></p>
<p>
<img alt="GitHub language count" src="https://github.com/Adelittle/GOIC/blob/main/image/goic1.png?raw=true">
</p>
</div>

GOIC (Go Orbit Ion Cannon) is not just another curl loop. It's a sophisticated, browser-based interface for configuring and executing high-concurrency HTTP requests. Built for developers, pentesters, and sysadmins, GOIC provides real-time feedback and fine-grained control for stress testing your web applications or APIs

The powerful Go backend ensures minimal resource consumption while delivering maximum throughput, allowing you to simulate heavy traffic from a single machine.

<p align="center">
<img src="https://github.com/Adelittle/GOIC/blob/main/image/goic2.png?raw=true" style="border-radius: 8px;" />
</p>
✨ Key Features

    🚀 High-Performance Backend: Written in Go (Golang) to handle thousands of concurrent connections with a minimal memory footprint.

    💻 Intuitive Web Interface: Control everything from a sleek, responsive dashboard. No command-line-fu required.

    📈 Real-time Monitoring: Watch the action unfold with live statistics (requests sent, success, failed) and a real-time log stream, all powered by WebSockets.

    ⚙️ Deeply Configurable: Take full control of your tests:

        [+] Target URL & HTTP Method (GET, POST, PUT, DELETE).

        [+] Concurrent Threads to simulate multiple users.

        [+] Request Count (fixed amount or infinite loop).

        [+] Custom User-Agents.

        [+] Inter-request Delay (in milliseconds).

        [+] Request Timeout.

    🔒 Secure by Design: Features token-based authentication for all API endpoints and a secure logout mechanism to prevent session fixation.

    🔄 Persistent Sessions: Your login session is saved, so you can refresh the page and pick up right where you left off.

    🤖 Automated Installation: A smart run.sh script handles everything from dependency checks to configuration and deployment.

⚠️ Disclaimer

This tool is intended for educational purposes and for testing applications you have explicit permission to test. Unauthorized use of this tool to attack targets is illegal and strictly prohibited. The developers assume no liability and are not responsible for any misuse of this tool.
🚀 Getting Started

Getting GOIC up and running is as simple as running a single script.
Prerequisites

    A Linux-based system (Debian/Ubuntu recommended).

    wget, curl, and tar (usually pre-installed).

    Root or sudo privileges.

Installation & Launch

    git clone https://github.com/Adelittle/GOIC/
    cd GOIC
    bash run.sh

Docker Install

    git clone https://github.com/Adelittle/GOIC/
    docker-compose up --build -d

    
    Login URL  https://<ip>:8082/ 
    user : root
    Password : toor
<div align="center">
<p>Made with ❤️ nakanosec.com.</p>
</div>
