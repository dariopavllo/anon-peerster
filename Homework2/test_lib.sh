RED='\033[31;1m'
YELLOW='\033[0;32m'
NC='\033[0m'

message_c1_1=Weather_is_clear
message_c2_1=Winter_is_coming
message_c3_1=In_the_beginning
message_c1_2=No_clouds_really
message_c2_2=Let\'s_go_skiing
message_c3=Is_anybody_here?

#testing
fail(){
	log_error "!!! Failed test: $1 ***"
  exit 1
}

wait_grep(){
	local file=$1/gossip.log
	local txt=$2
	shift
	while ! egrep -q "$txt" $file; do
		log_info "Didn't find yet --$txt-- in file: $file"
		sleep 1
  done
  log_info "Found -$txt- in file: $file"
}

test_grep(){
	local file=$1/gossip.log
	local txt=$2
	shift
	egrep -q "$txt" $file || fail "Didn't find --$txt-- in file: $file"
  log_info "Found -$txt- in file: $file"
}

test_ngrep(){
	local file=$1/gossip.log
	local txt=$2
	shift
	egrep -q "$txt" $file && fail "DID find --$txt-- in file: $file"
  log_info "Correct absence of -$txt- in file: $file"
}

log(){
	echo -e "\n$YELLOW*** $@$NC"
}
log_info(){
  echo " * $@"
}
log_error(){
	echo -e "\n$RED*** $@$NC"
}

denat(){
	iptables -t nat -F POSTROUTING
	iptables -F FORWARD
	iptables -P FORWARD ACCEPT
}

crosscompile(){
  if [ -z "$NOCOMP" ]; then
  	log "Cross-compiling gossiper and client-injector"
  	GOOS=linux GOARCH=386 go build
  	cd client
  	GOOS=linux GOARCH=386 go build
  	cd ..
  	mv part2 gossiper
  else
  	log "Not compiling gossiper and client!"
  fi
}

restartdocker(){
  cp ../docker/docker-compose.yaml .
	stopdocker ""
  rm -rf nodea nodeb nodepub
  log "Starting docker-compose"
  docker-compose up -d
  log "Waiting for docker-compose to start"
  while [ $( docker ps | wc -l ) != 6 ]; do
    sleep 2
  done
}

stopdocker(){
	local ns=${1:-$NOSTOP}
	if [ -z "$ns" ]; then
		running=$( docker ps -aq )
		if [ "$running" ]; then
			log "Stopping dockers"
			docker rm -f $running
		fi
	fi
}

startnodes(){
  log "Starting nodes in docker-containers"
  # Copying binaries to docker and run the gossiper
  for a in $@; do
    cp gossiper gossiper.sh client/client $a
  	noforward=""
  	if [ $a = nodepub ]; then
  		noforward="-noforward"
  	fi
  	docker exec -d $a /root/gossiper.sh $noforward
  done
  iptables -t nat -F POSTROUTING
  iptables -F FORWARD
  iptables -P FORWARD DROP
  iptables -A FORWARD -m state --state RELATED,ESTABLISHED -j ACCEPT 
  iptables -I FORWARD -s 172.16.0.0/24 -d 172.16.0.0/24 -j ACCEPT
  iptables -I FORWARD -s 172.16.1.0/24 -j ACCEPT
  iptables -I FORWARD -s 172.16.2.0/24 -j ACCEPT
}

sendmsg(){
  local node=$1
  shift
  local msg="$@"
  docker exec $node /root/client -msg="$msg"
}

sendpriv(){
	local node=$1
	local dst=$2
  shift 2
  local msg="$@"
  docker exec $node /root/client -msg="$msg" -Dest=$dst
}

for p in "$@"; do
  case "$p" in
    nc|nocomp)
      NOCOMP=true
      ;;
		ns|nostop)
		  NOSTOP=true
			;;
    *)
      echo "Unknown option"
      exit 1
  esac
done
