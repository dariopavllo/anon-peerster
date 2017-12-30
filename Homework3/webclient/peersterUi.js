let timer = null
let tabCounter = 2

$(document).ready(function(){
	update()
	timer = setInterval(update, 1000)
	
	$.fn.exists = function() {
		return this.length !== 0
	}
	
	$("#sendMessage").click(function() {
		const msg = $("#message").val()
		
		// Check selected tab
		const nodeName = $('li[aria-selected="true"]').attr("data-nodename")
		if (typeof nodeName !== typeof undefined && nodeName !== false) {
			// Private message
			$.ajax({
				type: 'POST',
				url: "/privateMessage",
				data: JSON.stringify({Destination: nodeName, Content: msg}),
				success: function() {
					update()
				},
				error: function() {
					alert("Unable to send private message")
				},
				contentType: "application/json"
			})
		} else {
			// Generic gossip message
			$.ajax({
				type: 'POST',
				url: "/message",
				data: JSON.stringify(msg),
				success: function() {
					update()
				},
				error: function() {
					alert("Unable to send gossip message")
				},
				contentType: "application/json"
			})
		}
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
	
	$("#searchFiles").click(function(){
		const keywords = $("#keywords").val()
		const budget = $("#budget").val()
		const searchRes = document.getElementById("tabs-search")
		searchRes.innerHTML = ""
		
		
		const elem = document.createElement("div")
				elem.appendChild(document.createTextNode("Search started..."))
				searchRes.appendChild(elem)
		
		// Supports incremental results
		let lastResponseLen = false
		$.ajax({
			type: 'POST',
			url: "/search",
			data: JSON.stringify({Keywords: keywords, Budget: budget}),
			xhrFields: {
				onprogress: e => {
					let thisResponse, response = e.currentTarget.response
                    if (lastResponseLen === false) {
                        thisResponse = response
                        lastResponseLen = response.length
                    } else {
                        thisResponse = response.substring(lastResponseLen)
                        lastResponseLen = response.length
                    }
					chunks = thisResponse.split("\n")
					chunks.forEach(chunk => {
						if (chunk.length == 0) {
							return
						}
						const elem = document.createElement("div")
						
						// Augment result by making it clickable
						const dlMatch = "Downloadable match: "
						if (chunk.startsWith(dlMatch)) {
							const str = chunk.substr(dlMatch.length).split(":")
							elem.appendChild(document.createTextNode(dlMatch))
							const fileLink = document.createElement("button")
							fileLink.append(document.createTextNode(chunk.substr(dlMatch.length)))
							$(fileLink).click(function() {
								$("input[name=fileName]").val(str[0])
								$("input[name=fileHash]").val(str[1])
								$("input[name=filePeer]").val("")
								document.getElementById("downloadForm").submit()
							})
							elem.appendChild(fileLink)
						} else {
							elem.appendChild(document.createTextNode(chunk))
						}
						searchRes.appendChild(elem)
					})
                }
			},
			success: function() {
				const elem = document.createElement("div")
				elem.appendChild(document.createTextNode("Search completed"))
				searchRes.appendChild(elem)
			},
			error: function() {
				alert("Unable to send search request")
			},
			dataType: "text"
		})
    })
	
	$("#tabs").tabs()
})

function update() {
	$.when(
		$.get("/id"),
		$.get("/node"),
		$.get("/message"),
		$.get("/routes")
	)
	.then(function(id, nodes, messages, routes) {
		$(".nodeName").text(JSON.parse(id[0]))
		const chatBox = document.getElementById("tabs-1")
		if (chatBox !== null) {
			chatBox.innerHTML = ""
			JSON.parse(messages[0]).forEach(m => {
				const elem = document.createElement("div")
				const nameTag = document.createElement("span")
				const date = m.FirstSeen.slice(0, 10)
				nameTag.appendChild(document.createTextNode(date + " | " + m.FromNode + " (relay: " + m.FromAddress + ") (ID: " + m.SeqID + "): "))
				elem.appendChild(nameTag)
				elem.appendChild(document.createTextNode(m.Content))
				chatBox.appendChild(elem)
			})
		}
		
		const peerBox = document.getElementById("peerContent")
		if (peerBox !== null) {
			peerBox.innerHTML = "<h2>Peers</h2>"
			JSON.parse(nodes[0]).sort((x, y) => x.Address.localeCompare(y.Address)).forEach(n => {
				const elem = document.createElement("div")
				const deleteButton = document.createElement("span")
				deleteButton.appendChild(document.createTextNode("(X) "))
				$(deleteButton).click(function(){
					$.ajax({
						type: 'POST',
						url: "/node",
						data: JSON.stringify(n.Address),
						success: function() {
							update()
						},
						error: function() {
							alert("Unable to delete peer")
						},
						contentType: "application/json"
					})
				})
				let description = ""
				switch (n.Type) {
					case 0:
						description = "manual"
						break
					case 1:
						description = "learned"
						break
					case 2:
						description = "short-circuited"
						break
				}
				elem.appendChild(deleteButton)
				elem.appendChild(document.createTextNode(n.Address + " (" + description + ")"))
				peerBox.appendChild(elem)
			})
		}
		
		const routeBox = document.getElementById("routeContent")
		if (routeBox !== null) {
			routeBox.innerHTML = "<h2>Known nodes</h2>"
			JSON.parse(routes[0]).sort((x, y) => x.Origin.localeCompare(y.Origin)).forEach(route => {
				const elem = document.createElement("div")
				const selectNode = document.createElement("span")
				selectNode.classList.add("button")
				selectNode.appendChild(document.createTextNode(route.Origin))
				$(selectNode).click(function() {
					if (!$('*[data-nodename="'+ route.Origin +'"]').exists()) {
						$("#tabs ul").append('<li data-nodename="' + route.Origin + '"><a href="#tabs-' + tabCounter + '">' + route.Origin + '</a> <span>x&nbsp;</span></li></ul>')
						$("#tabs").append('<div data-nodename="' + route.Origin + '" id="tabs-'+tabCounter+'"></div>')
						$("#tabs").tabs("refresh")
						$("#tabs ul li span").click(function() {
							const name = $(this).parent("li").attr('data-nodename')
							$('*[data-nodename="'+ name +'"]').remove()
						})
						tabCounter++
					}
				})
				elem.appendChild(selectNode)
				elem.appendChild(document.createTextNode(" (through " + route.Address + ")"))
				routeBox.appendChild(elem)
				
				$('div[data-nodename="'+ route.Origin +'"]').each(function() {
					const that = $(this)
					$.ajax({
						type: 'GET',
						url: "/privateMessage",
						data: {'name': route.Origin},
						success: function(result) {
							that.html("")
							JSON.parse(result).forEach(m => {
								const elem = document.createElement("div")
								const nameTag = document.createElement("span")
								const date = m.FirstSeen.slice(0, 10)
								nameTag.appendChild(document.createTextNode(date + " | " + m.FromNode + " (relay: " + m.FromAddress + "): "))
								elem.appendChild(nameTag)
								elem.appendChild(document.createTextNode(m.Content))
								that.append(elem)
							})
						},
						error: function() {
							alert("Unable to get private messages")
						},
						contentType: "application/json"
					})
				})
				
			})
		}
		
	}, function() {
		//alert("Unable to connect")
	})
}
