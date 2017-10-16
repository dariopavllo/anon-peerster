let address = ""
let timer = null

$(document).ready(function(){
    $("#connect").click(function(){
		address = $("#connectionAddress").val()
		update()
    })
	
	$("#sendMessage").click(function() {
		const msg = $("#message").val()
		$.ajax({
			type: 'POST',
			url: "http://" + address + "/message",
			data: JSON.stringify(msg),
			success: function() {
				update()
			},
			error: function() {
				alert("Unable to send message")
			},
			contentType: "application/json"
		})
	})
	
	$("#addPeer").click(function(){
		const peer = $("#newPeerAddress").val()
		$.ajax({
			type: 'POST',
			url: "http://" + address + "/node",
			data: JSON.stringify(peer),
			success: function() {
				update()
			},
			error: function() {
				alert("Unable to add peer")
			},
			contentType: "application/json"
		})
    })
	
	$("#changeId").click(function(){
		const name = $("#newName").val()
		$.ajax({
			type: 'POST',
			url: "http://" + address + "/id",
			data: JSON.stringify(name),
			success: function() {
				update()
			},
			error: function() {
				alert("Unable to change name")
			},
			contentType: "application/json"
		})
    })
})

function update() {
	$.when(
		$.get("http://" + address + "/id"),
		$.get("http://" + address + "/node"),
		$.get("http://" + address + "/message")
	)
	.then(function(id, nodes, messages) {
		$("#connectionBox").hide()
		$("#applicationBox").show()
		$(".nodeName").text(JSON.parse(id[0]))
		const chatBox = document.getElementById("chatContent")
		chatBox.innerHTML = "<h1>Messages</h1>"
		JSON.parse(messages[0]).forEach(m => {
			const elem = document.createElement("div")
			const nameTag = document.createElement("span")
			const date = m.FirstSeen.slice(0, 10)
			nameTag.appendChild(document.createTextNode(date + " | " + m.FromNode + " (relay: " + m.FromAddress + ") (ID: " + m.SeqID + "): "))
			elem.appendChild(nameTag)
			elem.appendChild(document.createTextNode(m.Content))
			chatBox.appendChild(elem)
		})
		
		const peerBox = document.getElementById("peerContent")
		peerBox.innerHTML = "<h1>Peers</h1>"
		JSON.parse(nodes[0]).sort().forEach(n => {
			const elem = document.createElement("div")
			elem.appendChild(document.createTextNode(n))
			peerBox.appendChild(elem)
		})
		
		if (timer != null) {
			clearInterval(timer)
		}
		timer = setInterval(update, 1000)		
	}, function() {
		$("#connectionBox").show()
		$("#applicationBox").hide()
		if (timer != null) {
			clearInterval(timer)
			timer = null
		}
		alert("Unable to connect")
	})
}

