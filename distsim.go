// Este programa requiere 2 parámetros de entrada :
//      - Nombre fichero json de Lefs
//        - Número de ciclo final
//
// Ejemplo : ./censim  testdata/Ejemplo1ParaTests.rdp.subred0.json 5
package main

import (
	"centralsim"
	//"os"
	"comm_vector"	
	//"net"
	"time"
	"fmt"
	"strings"
	"strconv"
	"sort"
)
/*
	Dado una lista de transiciones devuelve un diccionario 
	tal que dic[id_trans] = IP_Maquina 
*/
func CrearDic(L *centralsim.TransitionList,ip string,M *map[centralsim.IndLocalTrans]string)(){
	//M:= make(map[centralsim.IndLocalTrans]string);

	for _,trans := range *L{
		id := trans.IiIndLocal;
		//fmt.Println(id);
		(*M)[id] = ip;
	}

	//return M;
}

/*
	Devuelve true si en L hay una transicion con ID = id
*/
func Esta(L* centralsim.TransitionList,id centralsim.IndLocalTrans)(bool){
	found := false;

	for _,trans := range *L{
		found = id == trans.IiIndLocal;
		if (found){
			break;
		}
	}

	return found;
}

/*
	Devuelve una lista con los ids de las transiciones a los que les enviaras eventos
	Origen S
	Destino L
*/
func CheckObjetivo(L *centralsim.TransitionList,S *centralsim.SimulationEngine)([]centralsim.IndLocalTrans){
	lefs := S.GetLefs();
	var I []centralsim.IndLocalTrans;
	//I := make([]centralsim.IndLocalTrans,1);
	for _,trans := range lefs.IaRed{
		if (trans.Ib_de_salida){
			//fmt.Println("DE SALIDA")
			//Los eventos generados estan en otra subred
			for _,elem := range trans.TransConstPul{
				trans2:= elem[0]; //Transicion objetivo
				if (trans2 < 0 ){
					trans2 = -1 * (int(trans2)+1);
				}
				
				found := Esta(L,centralsim.IndLocalTrans(trans2));
				if (found){
					I = append(I,centralsim.IndLocalTrans(trans2));
				}
			}
		}
	}
	return I;
}

/*
	Dada una lista de transiciones devuelve el
	tiempo conjunto de ejecucion de todas ellas
*/
func TiempoTotal(L centralsim.TransitionList)(int){
	acum:=0;
	for _,trans := range L {
		acum += int(trans.IiDuracionDisparo);
		
	}
	
	return acum;
}


/*
	Elimina de un slice el indice s
*/
func remove(slice []string, s int) []string {
    return append(slice[:s], slice[s+1:]...)
}
/*
	Encuentra el maximo valor en un slice
*/
func max(slice []int)(int){
	aux := slice[0]
	for _,elem := range slice[1:]{
		if (elem > aux){
			aux = elem;
		}
	}
	return aux;
}
/*
	Sincroniza el comienzo de la simulacion
*/
func sincronizar(C * comm_vector.Communicator,IPs []string){
	id := C.Id;
	if id == 0 {
		for _,ip := range IPs[1:]{
			C.Try(ip);
		}
	}else{
		C.Init_Receive();
	}
}
/*
	Se calcula el incremento del lookahead a enviar cuando la subred no tenga el token
	El valor obtenido se tendrá que sumar al tiempo actual
	//Además se crea un diccionario tal que 
	dic[Transition_id] = ip_maquina_encargada
*/
func obtener_Incremento_lookAhead(ms *centralsim.SimulationEngine,C *comm_vector.Communicator,IPs []string)(int,map[centralsim.IndLocalTrans]string,map[string]chan comm_vector.Msg,map[string]chan comm_vector.Msg){
	//IPs = remove(IPs,C.Id);
	//fmt.Println(C.Id,IPs);
	lefs := ms.GetLefs();
	//Enviamos transiciones a todos
	for i,ip := range IPs{
		if i!=C.Id{
			C.Send_List(ip,lefs.IaRed_AUX);
		}
	}
	var received comm_vector.Msg_List;
	mapa := make(map[centralsim.IndLocalTrans]string);
	
	/*
		Recibes la lista de las transiciones de todos los procesos
		Respondes Enviando un Evento
		La cte del evento sera el numero de transiciones a las que tu le enviaras eventos en el funturo
		De forma que si el proceso es respondido con un Evento con una cte > 0 significa que tu 
		subred es un canal de entrada de la subred de dicho proceso

		El tiempo del evento será el tiempo total que toman la ejecucion de tus transiciones
	*/
	var E centralsim.Event;
	chan_map_salida := make(map[string]chan comm_vector.Msg );
	for i:=0 ;i<len(IPs)-1;i++{
		received = C.Receive_List()
		CrearDic(&received.L,received.IP,&mapa);
		
		//fmt.Println(C.Id,mapa)
		lista := CheckObjetivo(&received.L,ms);
		//fmt.Println(C.Id,lista)
		objetivo := len(lista)
		E.SetCte(centralsim.TypeConst(objetivo));
		if objetivo > 0 {
			chan_aux := make(chan comm_vector.Msg,1)
			chan_map_salida[received.IP] = chan_aux;
		}
		TtotalLocal := TiempoTotal(lefs.IaRed_AUX);
		E.SetTiempo(centralsim.TypeClock(TtotalLocal));
		C.Send(received.IP,E);
	}

	var L_suma []int;
	canales_entrada := 0 ;
	var Msg_E comm_vector.Msg;
	chan_map_entrada := make(map[string]chan comm_vector.Msg );
	/*
		Recibes la respuesta de todos los procesos
		Si es un canal de entrada anotas el tiempo de su ejecucion para calcular despues el max
		Ademas aniades un canal para ese proceso en el diccionario de canales
	*/
	for i:=0 ;i<len(IPs)-1;i++{
		Msg_E = C.Receive();
		E := Msg_E.Evento;
		if (int(E.IiCte) > 0 ){
			canales_entrada++;
			chan_aux := make(chan comm_vector.Msg,1)
			chan_map_entrada[Msg_E.IP] = chan_aux;
			L_suma = append(L_suma,int(E.IiTiempo));
		}
	}
	/*
		El incremento del lookahead será el tiempo de todas las transiciones
		+ el maximo tiempo de las subredes de entrada
	*/
	max := max(L_suma);
	lookahead:= max + TiempoTotal(lefs.IaRed_AUX);
	return lookahead,mapa,chan_map_entrada,chan_map_salida;
}
//Devuelve el menor valor del slice
func min(slice[]int)(int){
	aux:=slice[0];
	for _,elem := range slice{
		if elem < aux{
			aux = elem;
		}
	}
	return aux;
}


/*
	Calcula el incremento de lookahead cuando la subred tenga alguna transicion
	sensibilizada (haya token),el lookahead se calculara como el sumatorio
	de tiempos de transiciones desde la ultima transicion sensibilizada hasta
	la transicion de salida
*/
func Lookahead_Token(ms *centralsim.SimulationEngine)(int){
	var acum int;
	lefs:=ms.GetLefs();
	lefs.ActualizaSensibilizadas(0);
	if lefs.HaySensibilizadas(){
		TransMap := lefs.IaRed;
		keys := make([]int,len(TransMap));
		for key := range TransMap{
			keys = append(keys,int(key));
		}
		sort.Ints(keys);
		found := false; //Encontrada la transicion sensibilizada
		i:= 0 ;
		for !found{
			found = TransMap[centralsim.IndLocalTrans(keys[i])].IiValorLef <=0;
			if !found{
				i++;
			}
		}	
		//i es el indice de la primera transicion sensibilizada
		//Inicializamos el acumulador de tiempo a 0
		acum = int(TransMap[centralsim.IndLocalTrans(keys[i])].IiDuracionDisparo) ;
		salida := TransMap[centralsim.IndLocalTrans(keys[i])].Ib_de_salida;
		indice:=keys[i];
		for !salida{
			transicion := TransMap[centralsim.IndLocalTrans(indice)]
			salida = transicion.Ib_de_salida;
			
			//Si esta sensibilizada se vuelve a empezar
			if transicion.IiValorLef <=0{
				acum = 0 ;
			}

			//Aumentamos tiempo
			acum += int(transicion.IiDuracionDisparo);

			if !salida{
				trans_obj := transicion.TransConstPul;
				//Esto no deberia pasar en este problema
				if len(trans_obj) > 1{
					fmt.Println("MAS DE UNA TRANSICION OBJETIVO");
				}
				T_siguiente := trans_obj[0];
				indice = T_siguiente[0];

			}

			
		}

	

	}else{
		fmt.Println("NO HAY SENSIBILIZADAS EN LOOKAHEAD TOKEN");
		acum = -1;
	}
	return acum;

}
/*
	Comprueba si hay al menos un mensaje en el buffer de cada 
	canal
*/
func hay_uno(M *map[string]chan comm_vector.Msg)(bool){
	aux:= true;
	for _,channel := range *M{
		if aux{
			aux = len(channel) >0;
		}
	}
	return aux;
}
/*
	Recibe mensaje y lo envia al canal correspondiente
*/
func Recibir_Mensajes(C *comm_vector.Communicator,M *map[string]chan comm_vector.Msg){
	for !hay_uno(M){
		received := C.Receive();
		(*M)[received.IP]<-received
	}
}
/*
	Recibe los eventos de los canales
*/
func sacarEventos(M *map[string]chan comm_vector.Msg)([]centralsim.Event){
	var aux []centralsim.Event;
	for _,channel := range *M{
		msg := <-channel;
		aux = append(aux,msg.Evento);
	}
	return aux;

}
/*
	IPs -> slice con las IP de TODOS los procesos
	filename_json -> Nombre del fichero json a cargar
	id -> id del proceso, corresponde al indice de la IP del proceso en IPs
*/
func Simulador(IPs []string,filename string,id int){
	//Creamos el objeto communicator
	C:=comm_vector.Communicator{};
	puerto:=strings.Split(IPs[id],":")[1]
	file_log := "Proceso_"+strconv.Itoa(id);
	C.Init(IPs[id],puerto,file_log,id);

	//Creamos el simulation engine
	lefs, err := centralsim.Load(filename);
	if err != nil {
		println("Couln't load the Petri Net file !")
	}
	ms := centralsim.MakeMotorSimulation(lefs)

	//Se sincroniza el evento de la simulacion
	sincronizar(&C,IPs);


	//Obtenemos los lookahead
	//lookahead[0]->Si la subnet tiene el token
	//lookahead[1]->Si la subnet no tiene el token
	lookahead,mapa_trans,mapa_chan_entrada,mapa_chan_salida := obtener_Incremento_lookAhead(&ms,&C,IPs);


	time.Sleep(time.Duration(C.Id) * 200 * time.Millisecond);
	fmt.Println(C.Id);
	fmt.Println("=====================")
	fmt.Println("Mi lookahead cuando no tengo token: ",lookahead);
	for trans_id,ip := range mapa_trans{
		fmt.Println("Transicion ",trans_id," en ",ip)
	}
	fmt.Println("Canales de salida: ");
	for key,_ := range mapa_chan_salida{
		fmt.Println(key);
	}
	fmt.Println("Canales de entrada: ");
	for key,_ := range mapa_chan_entrada{
		fmt.Println(key);
	}
	if C.Id == 0{
		fmt.Println("Mi lookahead cuando tengo token",Lookahead_Token(&ms));
	}

	/*
		Actualizar sensibilizadas
		for
			Enviar lookahead
				-Comprobar que lookahead te toca
				-Enviar a todos los canales de salida
			Recibir lookaheads
			Procesar Eventos
			Simular lo correspondiente
		
	*/
	Recibir_Mensajes(&C,&mapa_chan_entrada);
	Eventos_recibidos := sacarEventos(&mapa_chan_entrada);

	//fmt.Printf(filename,lookahead);
	//fmt.Printf(filename,mapa_trans);
	//fmt.Printf(filename,mapa_chan);
}

func Init(IPs []string,filename string,id int){
	Simulador(IPs,filename,id);
	time.Sleep(1000 * time.Millisecond);
}
func main() {
	// cargamos un fichero de estructura Lef en formato json para centralizado
	// os.Args[0] es el nombre del programa que no nos interesa
	IPs := []string{"localhost:30000","localhost:40000","localhost:50000"};
	subredes := "./testdata/3subredes.subred";
	for i,_ := range IPs{
		filename := subredes+strconv.Itoa(i)+".json";
		if i != len(IPs)-1 {
			go Init(IPs,filename,i);
		}else{
			Init(IPs,filename,i)
		}
	}
	
}
