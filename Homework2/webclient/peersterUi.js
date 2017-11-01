let timer = null

$(document).ready(function(){
	update()
	timer = setInterval(update, 1000)
	
	$("#sendMessage").click(function() {
		const msg = $("#message").val()
		$.ajax({
			type: 'POST',
			url: "/message",
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
			url: "/node",
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
			url: "/id",
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
		$.get("/id"),
		$.get("/node"),
		$.get("/message")
	)
	.then(function(id, nodes, messages) {
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
	}, function() {
		alert("Unable to connect")
	})
}

