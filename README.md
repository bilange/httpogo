# Http Web Server (HTTPogo)


### What's with the funny name?! 
"*Concordantly, while your first question may be the most pertinent, you may or may not realize it is also the most irrelevant.*" -The Architect, The Matrix Reloaded (2003)

It looks like a good share of the Go projects out there has some 'Go' pun lying around. As for why 'pogo': this web server is kinda like eating pogos. It may not be the most efficient thing out there, but it does the job. 

### Features
Here's a rundown of the features (so far) in this server: 

* Virtual-hosts support: subdirectories (from where your httpogo executable resides) are treated as v-hosts root folders. For example, if you modify your operating systems [hosts file](http://en.wikipedia.org/wiki/Hosts_%28file%29) for local development (For example: http://test.dev), the subfolder "test.dev" will be served as your public root folder under http://test.dev
  * Of course, this works with FQDNs too. Also with subdomains.
  * If you don't care about v-hosts and want a quick-and-dirty http localhost server, you can also just using the subdirectory "public_html" instead (which is configurable by a command-line parameter). 
  * You can also symlink from a subdirectory to another, as you can expect.
* Basic Auth support: You can "lock" directories (recursively) by adding an ".auth" file in that directory. Every line of that file is using the "username:password" format, and protects the directory's content (child dirs too) from being viewed publicly. NOTE: This is insecure. Do not use sensitive passwords, as it is sent clear-text, as per HTTP protocol. See [this page](http://en.wikipedia.org/wiki/Basic_access_authentication) for info.
* PHP Support: this server permits some extent of PHP support, provided that you copy your own php-cgi file in the files/ subdirectory. PHP files with lowercase ".php" extension will be treated by PHP via CGI (not FastCGI, i'm afraid)
* Dumb executable handler support: as a worst case scenario, if you have an executable (say, a compiled C file, or even a bash script), you can get a raw HTTP request from stdin, and output a raw HTTP response over stdout, simple as that. (**Warning**: this might be insecure, wrong, or otherwise frowned upon. Don't use this trick in a production server (even less so on the Internets))
* Logging support, using the Apache access_log and error_log file format
* Markdown support for files haiving the .md extension, using the excellent [Blackfriday](https://github.com/russross/blackfriday) Markdown to HTML generator.
* Templates for HTTP 401-3-4 error codes, open directories and Markdown, all overridable. For HTTP 4xx templates, I provided a bilingual (French & English) template, style is based upon (well, ripped, really) Blackfriday's Markdown HTML style.


### Usage
Just run `./httpogo` from the command-line, and keep the server running in the console (as far as I know, Go programs cannot programmatically fork into background without using shell hacks*), and accept the defaults settings, which are: 

* Print out ERRORS on the console for debugging purposes (warnings will be silenced out)
* Listen on port 80 (see related note below)
* The program will use `.` (the program's directory) as the root folder. You can fine-tune this setting in case the 'files', 'logs' and vhosts subfolders are placed elsewhere, or have other specific needs.
* Use the `public_html` subdirectory when no requested hostnames matches

Once running, point your browser to the IP/Port where you listen for connections (this program listens on every IPs the machine has, using the port you specified), and develop away!

\* To (try to) launch your webserver in the background, I was able to launch it using Ubuntu's upstart service with this command: 

`exec sudo -u USERNAME -g GROUP nohup /path/to/httpogo [HTTPOGO OPTIONS] >/dev/null 2>/dev/null &`

The `sudo -u USERNAME -g GROUP` part is merely to ensure that the program won't have root priviledges when running.

I assume this hack also works with other job systems like init scripts, etc. Don't hesitate to prove me wrong.

#### Running httpogo with a port number under 1024 
For Unix systems, ports under 1024 are reserved for root. Of course, running your application under root is a security flaw, [that's where Stackoverflow comes in and saves the day](http://stackoverflow.com/a/414258) :) (I am unaware if this is a resolvable issue for OSx. Sorry. I also assume that it would work on Windows machines)

### License, Legal, Etc
httpogo is distributed under the Simplified BSD License:

> Copyright © 2012 Eric Belanger. All rights reserved.
> 
> Redistribution and use in source and binary forms, with or without modification, are
> permitted provided that the following conditions are met:
> 
>    1. Redistributions of source code must retain the above copyright notice, this list of
>       conditions and the following disclaimer.
> 
>    2. Redistributions in binary form must reproduce the above copyright notice, this list
>       of conditions and the following disclaimer in the documentation and/or other materials
>       provided with the distribution.
> 
> THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDER ``AS IS'' AND ANY EXPRESS OR IMPLIED
> WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND
> FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL ERIC BELANGER OR
> CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
> CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
> SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
> ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING
> NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF
> ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
> 
> The views and conclusions contained in the software and documentation are those of the
> authors and should not be interpreted as representing official policies, either expressed
> or implied, of the copyright holder.
