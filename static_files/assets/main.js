function getStatus() {
    return fetch("/mpd/status").then((response) => {
        return response.json();
    });
}

function musicNext() {
    fetch("/mpd/next");
}

function musicToggle() {
    getStatus().then((mpdstatus) => {
        if (mpdstatus.state === "play") {
            fetch("/mpd/pause 1");
        } else {
            fetch("/mpd/pause 0");
        }
    });
}

function musicConsume() {
    getStatus().then((mpdstatus) => {
        if (mpdstatus.consume === "1") {
            fetch("/mpd/consume 0");
        } else {
            fetch("/mpd/consume 1");
        }
        document.getElementById("consume").style.opacity =
            ((mpdstatus.consume === "1")? "0.3" : "1");
    });
}

function musicRepeat() {
    getStatus().then((mpdstatus) => {
        if (mpdstatus.repeat === "1") {
            fetch("/mpd/repeat 0");
        } else {
            fetch("/mpd/repeat 1");
        }

        document.getElementById("repeat").style.opacity =
            ((mpdstatus.repeat === "1")? "0.3" : "1");
    });
}

function musicPrev() {
    fetch("/mpd/previous");
}

function getCurrentSong() {
    return fetch("/mpd/currentsong").then((response) => {
        return response.json();
    });
}

function musicUpdate() {

    getCurrentSong().then((currentsong) => {
        document.getElementById("song-title").textContent = currentsong.Title;
        document.getElementById("song-artist").textContent = currentsong.Artist;
        document.getElementById("song-album").textContent = currentsong.Album;

        document.getElementById("coverimg").setAttribute("src", "/cover?A=" + currentsong.Album);

    });

    getStatus().then((mpdstatus) => {
        fetch("/mpd/playlistid " + mpdstatus.nextsongid).then((response) => {
            response.json().then((data) => {
                document.getElementById("song-title-next").textContent = data.Title;
                document.getElementById("song-artist-next").textContent = data.Artist;
                document.getElementById("song-album-next").textContent = data.Album;
            });

            var play = document.getElementById("toggle").firstChild;
            if (mpdstatus.state === "play") {
                play.classList.add("fa-pause");
                play.classList.remove("fa-play");
            } else {
                play.classList.add("fa-play");
                play.classList.remove("fa-pause");
            }

            document.getElementById("random").style.opacity =
                ((mpdstatus.random === "1")? "1" : "0.3");

            document.getElementById("consume").style.opacity =
                ((mpdstatus.consume === "1")? "1" : "0.3");

            document.getElementById("repeat").style.opacity =
                ((mpdstatus.repeat === "1")? "1" : "0.3");
        });
    });
}

function musicRandom() {
    getStatus().then((mpdstatus) => {
        if (mpdstatus.random === "1") {
            fetch("/mpd/random 0");
        } else {
            fetch("/mpd/random 1");
        }
        musicUpdate();
    });
}

function playsong(songid) {
    fetch("/mpd/play " + songid);
}

function populatePlaylist() {

    getStatus().then((mpdstatus) => {
        fetch("/mpd/playlist").then((response) => {
            response.json().then((data) => {


                const orderedData = {};
                Object.keys(data).sort(function(a, b) {
                    const aI = parseInt(a, 10);
                    const bI = parseInt(b, 10);
                    if (aI > bI) {
                        return 1;
                    } else if ( aI < bI ) {
                        return -1;
                    }
                    return 0;
                }).forEach(function(key) {
                    orderedData[key] = data[key];
                });

                var list = document.createElement("ul");

                list.className = "list-group";

                for (var elem in orderedData) {
                    if (orderedData.hasOwnProperty(elem)) {
                        var item = document.createElement("li");

                        item.appendChild(document.createTextNode(
                            data[elem].replace(/\.[^/.]+$/, "").replace(/\//g, " - "
                            )));

                        var songNum = parseInt(elem, 10);

                        item.className = "list-group-item";
                        item.setAttribute("onclick", "playsong('" + songNum + "')");
                        item.addEventListener("mousedown", function(e){ e.preventDefault(); }, false);

                        if (songNum === parseInt(mpdstatus.song, 10)) {
                            item.className = "list-group-item active";
                        }

                        list.appendChild(item);
                    }
                }

                var PlaylistOld = document.getElementById("playlist");

                var PlaylistParent = PlaylistOld.parentNode;

                var PlaylistNew = document.createElement("div");
                PlaylistNew.id = "playlist";
                PlaylistNew.appendChild(list);
                PlaylistParent.insertBefore(PlaylistNew, PlaylistOld);

                PlaylistParent.removeChild(PlaylistOld);

            });
        });
    });
}

function mixPlaylist() {
    fetch("/mpd/shuffle");
    musicUpdate();
    populatePlaylist();
}

window.onload = function() {
    musicUpdate();
    populatePlaylist();
};

document.addEventListener("visibilitychange", function() {
    if (document.visibilityState === "visible") {
        //console.log("Visibility changed");

        musicUpdate();
        populatePlaylist();
    }
});


var eSource = new EventSource("sse");
eSource.onmessage = function(event) {
    musicUpdate();
    populatePlaylist();
};

window.onbeforeunload = function() {
    eSource.close();
};

document.getElementById("prev").addEventListener("click", musicPrev);
document.getElementById("toggle").addEventListener("click", musicToggle);
document.getElementById("next").addEventListener("click", musicNext);

document.getElementById("next-song").addEventListener("click", musicNext);
document.getElementById("next-song").addEventListener("mousedown", function(e){ e.preventDefault(); }, false);

document.getElementById("coverimg").addEventListener("click", musicToggle);

document.getElementById("repeat").addEventListener("click", musicRepeat);
document.getElementById("consume").addEventListener("click", musicConsume);
document.getElementById("random").addEventListener("click", musicRandom);

document.getElementById("shuf-play").addEventListener("click", mixPlaylist);
