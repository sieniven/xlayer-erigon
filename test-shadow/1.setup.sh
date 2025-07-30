set -e 


tdir=$(pwd)/tmp
if [ ! -d "$tdir" ]; then
    echo "Creating working directory structure..."
    mkdir  -p "$tdir/anvil"
    mkdir  -p "$tdir/agglayer"
    mkdir  -p "$tdir/aggkit"
    chmod -R 777 "$tdir"
fi

if [ ! -d "aggkit-code" ]; then
    git clone git@github.com:okx/aggkit.git aggkit-code
    cd aggkit-code
    git checkout feature/0.1.0
    make build-docker
else
    cd aggkit-code
    git checkout feature/0.1.0
    git pull
    make build-docker
fi
cd "$tdir"

if [ ! -d "agglayer-contracts" ]; then
    git clone git@github.com:agglayer/agglayer-contracts.git
    git checkout v11.0.0-rc.0
else 
    cd agglayer-contracts
    rm -rf *; git reset --hard; 
    git pull;  
    git checkout v11.0.0-rc.0
fi
cd "$tdir"

fi





