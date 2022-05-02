Summary: netclient a platform for modern, blazing fast virtual networks
Name: netclient
Version: VERSION
Release: 1
URL: https://github.com/gravitl/netmaker
Group: System
License: SSPL # https://github.com/gravitl/netmaker/blob/master/LICENSE.txt
Packager: Gravitl
Requires: wireguard-tools
BuildRoot: /root/rpmbuild # this should be replaced with your working directory where the spec is saved

%description
netclient daemon - a platform for modern, blazing fast virtual networks

%install
mkdir -p %{buildroot}/usr/sbin/
mkdir -p %{buildroot}/usr/lib/systemd/system
wget https://github.com/gravitl/netmaker/releases/download/vVERSION/netclient -O $RPM_BUILD_ROOT/usr/sbin/netclient
wget https://raw.githubusercontent.com/gravitl/netmaker/master/netclient/build/netclient.service -O $RPM_BUILD_ROOT/usr/lib/systemd/system/netclient.service

%files
/usr/sbin/netclient
/usr/lib/systemd/system/netclient.service

%changelog
* Mon May 1 2022 <info@gravitl.com>
- What's New

    Instant DNS propogation

What's Fixed

    IPv6 forwarding working from ext clients to nodes
    netclient list displays peer info again
    Fixed indefinite hang on netclient join, attempts to pull certificates

Known Issues

    Egress with IPv6 may have issues
    Mac IPv6 routes not resolved
    Windows install script not fixed
 