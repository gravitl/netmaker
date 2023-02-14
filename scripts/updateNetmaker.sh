#! /bin/sh
if (($# < 3))
then
	echo "you should have three arguments. netmaker branch, netmaker-ui branch, and version"
	exit
elif (($# > 3))
then
	echo "you should have three arguments. netmaker branch, netmaker-ui branch, and version"
	exit
else
	echo "pulling netmaker branch $1, netmaker-ui branch $2, and version will be $3."
fi
docker images
#docker images -a | grep theguy | awk '{ print $3; }' | xargs docker rmi --force
docker images -a | grep none | awk '{ print $3; }' | xargs docker rmi --force
docker images -a | grep alpine | awk '{ print $3; }' | xargs docker rmi --force
docker images -a | grep gravitl | awk '{ print $3; }' | xargs docker rmi --force
docker images -a | grep nginx | awk '{ print $3; }' | xargs docker rmi --force
docker images -a | grep node | awk '{ print $3; }' | xargs docker rmi --force
docker images
git clone https://www.github.com/gravitl/netmaker
git clone https://www.github.com/gravitl/netmaker-ui
cd netmaker
git checkout $1
git pull origin $1
go mod tidy
wait
docker build --no-cache --build-arg version=$3 -t gravitl/netmaker:testing .
wait
docker push gravitl/netmaker:testing
wait
docker build --no-cache --build-arg version=$3 -t gravitl/netmaker:testing-ee --build-arg tags="ee" .
wait
docker push gravitl/netmaker:testing-ee
wait
echo "netmaker and netmaker enterprise updated with version $3, built and pushed"
cd
cd netmaker-ui
git checkout $2
git pull origin $2
go mod tidy
wait
docker build --no-cache -t gravitl/netmaker-ui:testing .
wait
docker push gravitl/netmaker-ui:testing
wait
echo "netmaker-ui updated, built, and pushed."
cd
rm -rf netmaker
rm -rf netmaker-ui

