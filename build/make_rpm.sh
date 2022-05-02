rpmbuild -bb netclient.spec
rpm --addsign rpmbuild/RPMS/x86_64/netclient-VERSION-1.x86_64.rpm
mkdir -p /var/rpm-repo/packages
cp /root/rpmbuild/RPMS/x86_64/netclient-VERSION-1.x86_64.rpm /var/rpm-repo/packages/.
cd /var/rpm-repo/packages
createrepo_c .
if test -f repodata/repomd.xml.asc; then
    rm repodata/repomd.xml.asc
fi
gpg --detach-sign --armor repodata/repomd.xml
if test -f /var/rpm-repo/gpg.key; then
    rm /var/rpm-repo/gpg.key
fi
gpg --export -a --output /var/rpm-repo/gpg.key
cat <<EOF > /var/rpm-repo/netclient-repo
[netclient-repo]
name=netclient 
baseurl=https://rpm.netmaker.org/packages
enabled=1
pgpcheck=1
pgpkey=https://rpm.netmaker.org/gpg.key
EOF
