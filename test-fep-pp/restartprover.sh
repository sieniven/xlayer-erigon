docker-compose stop xlayer-aggkit;docker-compose stop xlayer-agglayer; docker-compose stop xlayer-agglayer-prover;
docker rm xlayer-aggkit;docker rm xlayer-agglayer; docker rm xlayer-agglayer-prover;

sleep 5
rm -rf data/aggkit/ ; rm -rf data/agglayer/ ;

sleep 5
docker-compose up xlayer-agglayer-prover -d

sleep 5
docker-compose up xlayer-agglayer -d

sleep 5
mkdir -p data/aggkit
chmod -R 777 data 
docker-compose up xlayer-aggkit -d