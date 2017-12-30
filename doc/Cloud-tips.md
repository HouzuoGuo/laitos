# Cloud tips

## General information
laitos runs well on all popular cloud vendors such as Amazon Web Service, Microsoft Azure, and Google Compute Engine.

In fact, it works well on nearly all public and private cloud vendors, irrespective of computer form factor,
virtualisation technology, hardware model, and administration interface.

## Manually integrate with systemd
systemd is the most popular init system for Linux distributions, it can help launching laitos automatically when
computer starts up. Create a service file `/etc/systemd/system/laitos.service` and write:

    [Unit]
    Description=laitos - web server infrastructure
    After=network.target

    [Service]
    ExecStart=/root/laitos/laitos -config /root/laitos/config.json -daemons dnsd,httpd,insecurehttpd,maintenance,plainsocket,smtpd,sockd,telegram
    User=root
    Group=root
    WorkingDirectory=/root/laitos
    PrivateTmp=true
    RestartSec=10
    Restart=always

    [Install]
    WantedBy=multi-user.target

Make sure to alter the paths in `ExecStart` and `WorkingDirectory` according to your setup. For security, please place
laitos program (e.g. `/root/laitos/laitos`) and its data directory (e.g. `/root/laitos`) in a location accessible only
by superuser `root`.

After the service file is in place, run these commands as root user:

    # systemctl daemon-reload    (Re-read all service files, including the new one)
    # systemctl enable laitos    (Remember to start laitos when system boots up)
    # systemctl start laitos     (tell systemd to start laitos immediately)

## Deploy on Amazon Web Service
In ordinary scenarios, simply copy laitos program and its data onto an EC2 instance and start laitos right away. It is
often useful to use systemd integration to launch laitos automatically upon system boot. All flavours of Linux
distributions supported by EC2 can run laitos.

For a fancier setup, Amazon Web Service offers Platform-as-a-Service called "ElasticBeanstalk" that deploys application
(laitos) on automatically managed EC2 instances. Here are some tips for using laitos on ElasticBeanstalk:
- For a personal web server, it is sufficient to use "Single Instance" as environment type. Load balancer is not
  necessary in this case.
- ElasticBeanstalk always expects their application to serve a web server on port 5000, otherwise it will consider
  an application (laitos) to be malfunctioning. If your laitos configuration has already set up web server, you must
  start `insecurehttpd` that will automatically listen on port 5000 when it detects ElasticBeanstalk environment.
  ElasticBeanstalk system will then serve (proxy) Internet visitors on port 80 using laitos web server on port 5000.
- If your configuration does not set up a web server, please set one up just for ElasticBeanstalk, or use the ordinary
  deployment method.
- ElasticBeanstalk application is packed into a zip file called "application bundle", that typically contains:

      Procfile                       (launch command line text)
      .ebextensions/options.config   (workaround - launch laitos as root)

      laitos       (program executable)
      config.json  (configuration file)

      index.html   (other data)
      ...

- `Procfile` tells the command line for starting laitos, the content may look like:

      laitos: ./laitos -config config.json -daemons dnsd,httpd,insecurehttpd,maintenance,plainsocket,smtpd,sockd,telegram

  Be aware that, paths among the command line must be relative to the top level of application bundle zip file.

- `.ebextensions/options.config` alters operating system configuration in order to launch laitos as user root.
  Otherwise ElasticBeanstalk launches program using an unprivileged user that will cause laitos to malfunction.
  The file shall contain the following content:

      ---
      commands:
        0_run_as_root:
          command: "find /opt/elasticbeanstalk -name 'app.conf.erb' -exec sed -i 's/^user=.*$/user=root/' {} \\;"
        1_reload_supervisor:
          command: "/usr/local/bin/supervisorctl -c /etc/supervisor/supervisord.conf reload"
          ignoreErrors: true

- Remember to adjust firewall (security group) to open ports for all services (e.g. DNS, SMTP, HTTPS) served by laitos.

## Deploy on Microsoft Azure and Google Compute Engine
Simply copy laitos program and its data onto a Linux virtual machine and start laitos right away. It is often useful to
use systemd integration to launch laitos automatically upon system boot. All flavours of Linux distributions supported
by Azure can run laitos.

Be aware that both Azure and GCE block virtual machines from _outgoing_ connections to port 25, that means while laitos
will be perfectly capable of receiving mails, it will not be able to forward them to you; therefore, for laitos outgoing
mail configuration, please sign up for a dedicated mail delivery service such as [sendgrid](https://sendgrid.com/)
that accepts connection to a port different from 25. Sendgrid accepts incoming connection on port 2525.

## Deploy on other cloud providers
laitos runs on nearly all flavours of Linux system, therefore as long as your cloud provider supports Linux compute
instance, you can be almost certain that it will run laitos smoothly and well.

Beyond AWS, Azure, and GCE, the author of laitos has also successfully deployed it on generic KVM virtual machine,
OpenStack, Linode, and several cheap hosting services advertised on [lowendbox.com](https://lowendbox.com/).