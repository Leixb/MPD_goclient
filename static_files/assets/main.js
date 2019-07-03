window.onload = function() {
    musicUpdate(); populatePlaylist();
}


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
document.getElementById("next-song").addEventListener('mousedown', function(e){ e.preventDefault(); }, false);

document.getElementById("coverimg").addEventListener("click", musicToggle);

document.getElementById("repeat").addEventListener("click", musicRepeat);
document.getElementById("consume").addEventListener("click", musicConsume);
document.getElementById("random").addEventListener("click", musicRandom);

document.getElementById("shuf-play").addEventListener("click", mixPlaylist);

function mixPlaylist() {
    fetch('/mpd/shuffle');
    musicUpdate();
    populatePlaylist();
}

function musicNext() {
    fetch('/mpd/next');
}
function musicToggle() {
    getStatus().then(mpdstatus => {
        if (mpdstatus.state == "play") {
            fetch('/mpd/pause 1');
        } else {
            fetch('/mpd/pause 0');
        }
    })
}

function musicRandom() {
    getStatus().then(mpdstatus => {
        if (mpdstatus.random == "1") {
            fetch('/mpd/random 0');
        } else {
            fetch('/mpd/random 1');
        }
        document.getElementById("random").style.opacity =
            ((mpdstatus.random == "1")? "0.3" : "1");
    })
}

function musicConsume() {
    getStatus().then(mpdstatus => {
        if (mpdstatus.consume == "1") {
            fetch('/mpd/consume 0');
        } else {
            fetch('/mpd/consume 1');
        }
        document.getElementById("consume").style.opacity =
            ((mpdstatus.consume == "1")? "0.3" : "1");
    })
}

function musicRepeat() {
    getStatus().then(mpdstatus => {
        if (mpdstatus.repeat == "1") {
            fetch('/mpd/repeat 0');
        } else {
            fetch('/mpd/repeat 1');
        }

        document.getElementById("repeat").style.opacity =
            ((mpdstatus.repeat == "1")? "0.3" : "1");
    })
}

function musicPrev() {
    fetch('/mpd/previous');
}

function getCurrentSong() {
    return fetch('/mpd/currentsong').then(response => {
        return response.json()
    })
}
function getStatus() {
    return fetch('/mpd/status').then(response => {
        return response.json()
    })
}

function musicUpdate() {

    getCurrentSong().then(currentsong => {
        document.getElementById("song-title").innerHTML = currentsong.Title;
        document.getElementById("song-artist").innerHTML = currentsong.Artist;
        document.getElementById("song-album").innerHTML = currentsong.Album;

        document.getElementById("coverimg").setAttribute("src", "/cover?A=" + currentsong.Album)

    });

    getStatus().then(mpdstatus => {
        fetch('/mpd/playlistid ' + mpdstatus.nextsongid).then(response => {
            response.json().then(data => {
                document.getElementById("song-title-next").innerHTML = data.Title;
                document.getElementById("song-artist-next").innerHTML = data.Artist;
                document.getElementById("song-album-next").innerHTML = data.Album;
            });

            var play = document.getElementById("toggle").firstChild;
            if (mpdstatus.state == "play") {
                play.classList.add("fa-pause");
                play.classList.remove("fa-play");
            } else {
                play.classList.add("fa-play");
                play.classList.remove("fa-pause");
            }

            document.getElementById("random").style.opacity =
                ((mpdstatus.random == "1")? "1" : "0.3");

            document.getElementById("consume").style.opacity =
                ((mpdstatus.consume == "1")? "1" : "0.3");

            document.getElementById("repeat").style.opacity =
                ((mpdstatus.repeat == "1")? "1" : "0.3");
        })
    });
}

function playsong(songid) {
    fetch('/mpd/play ' + songid);
}

function populatePlaylist() {

    getStatus().then(mpdstatus => {
        fetch('/mpd/playlist').then(response => {
            response.json().then(data => {

                var list = document.createElement('ul');

                list.className = "list-group"

                var data_list = new Array(data.length)

                for (elem in data) {
                    data_list[parseInt(elem)] = data[elem];
                }

                for (var i=0; i < data_list.length; ++i) {


                    var item = document.createElement('li');

                    item.appendChild(document.createTextNode(
                        data_list[i].replace(/\.[^/.]+$/, "").replace(/\//g, " - "
                        )));

                    item.className = "list-group-item";
                    item.setAttribute("onclick", "playsong('" + i + "')");
                    item.setAttribute("onclick", "playsong('" + i + "')");
                    item.addEventListener('mousedown', function(e){ e.preventDefault(); }, false);

                    if (i == mpdstatus.song) {
                        item.className = "list-group-item active";
                    }

                    list.appendChild(item);

                }

                var playlist_old = document.getElementById('playlist');

                var playlist_parent = playlist_old.parentNode;

                var playlist_new = document.createElement('div');
                playlist_new.id = "playlist";
                playlist_new.appendChild(list);
                playlist_parent.insertBefore(playlist_new, playlist_old);

                playlist_parent.removeChild(playlist_old);

            })
        })
    });
}
